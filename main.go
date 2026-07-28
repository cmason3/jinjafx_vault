/*
 * JinjaFx Vault
 * Copyright (c) 2026 Chris Mason <chris@netnix.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package main

import (
  "os"
  "io"
  "fmt"
  "net"
  "log"
  "time"
  "flag"
  "sync"
  "embed"
  "bytes"
  "io/fs"
  "bufio"
  "regexp"
  "strings"
  "strconv"
  "context"
  "syscall"
  "os/exec"
  "net/http"
  "os/signal"
  "path/filepath"
  "html/template"
  "github.com/creack/pty"
  "github.com/gorilla/websocket"
  "github.com/lithammer/shortuuid/v4"
)

const Version = "1.0.0"

type httpWriter struct {
  http.ResponseWriter
  remoteHost string
  statusCode int
}
func responseWriter(w http.ResponseWriter) *httpWriter {
  return &httpWriter { w, "", http.StatusOK }
}
func (w *httpWriter) WriteHeader(statusCode int) {
  w.statusCode = statusCode
  w.Header().Set("Cache-Control", "no-store")
  w.ResponseWriter.WriteHeader(w.statusCode)
}

func slog(f string, a ...any) {
  m := fmt.Sprintf(f, a...)
  if _, defined := os.LookupEnv("JOURNAL_STREAM"); defined {
    m = regexp.MustCompile(`\033\[(?:(?:[01];)?[0-9][0-9]|0)m`).ReplaceAllString(m, "")
  }
  log.Print(m)
}

func logRequest(h http.Handler, xffPtr bool) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    _w := responseWriter(w)
    remoteHost, _, _ := net.SplitHostPort(r.RemoteAddr)
    if xffPtr && r.Header.Get("X-Forwarded-For") != "" {
      _w.remoteHost = r.Header.Get("X-Forwarded-For")

    } else {
      _w.remoteHost = remoteHost
    }
    h.ServeHTTP(_w, r)

    if _w.statusCode > 0 {
      var statusCode string

      if _w.statusCode >= 400 {
        statusCode = fmt.Sprintf("\033[31m%d\033[0m", _w.statusCode)

      } else if _w.statusCode >= 300 {
        statusCode = fmt.Sprintf("\033[33m%d\033[0m", _w.statusCode)

      } else {
        statusCode = fmt.Sprintf("\033[32m%d\033[0m", _w.statusCode)
      }
      slog("[%s] {%s} %s %s\n", _w.remoteHost, statusCode, r.Method, r.URL.Path)
    }
  })
}

func wwwHandler(h http.Handler, tmpl *template.Template, eTag string, ft bool) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/" {
      r.URL.Path = "/index.html"

    } else if r.URL.Path == "/logs" {
      r.URL.Path = "/logs.html"

    } else if r.URL.Path == "/ft" {
      r.URL.Path = "/ft.html"
    }

    if r.Header.Get("If-None-Match") == eTag {
      w.WriteHeader(http.StatusNotModified)

    } else {
      if strings.HasPrefix(r.URL.Path, fmt.Sprintf("/%s/", eTag)) {
        r.URL.Path = strings.TrimPrefix(r.URL.Path[1:], eTag)
        w.Header().Set("Cache-Control", "max-age=31536000, immutable")

      } else {
        w.Header().Set("Cache-Control", "max-age=0, must-revalidate")
        w.Header().Set("ETag", eTag)
      }

      if t := tmpl.Lookup(r.URL.Path[1:]); t != nil {
        var buf bytes.Buffer

        data := map[string]string {
          "Version": eTag,
          "FileTransfer": fmt.Sprintf("%v", ft),
        }

        if err := t.Execute(&buf, data); err == nil {
          w.Write(buf.Bytes())

        } else {
          http.Error(w, err.Error(), http.StatusInternalServerError)
        }
      } else {
        h.ServeHTTP(w, r)
      }
    }
  })
}

func main() {
  if _, defined := os.LookupEnv("JOURNAL_STREAM"); !defined {
    fmt.Fprintf(os.Stdout, "WebTTY v%s - Remote Terminal\n", Version)
    fmt.Fprintf(os.Stdout, "URL https://github.com/cmason3/webtty\n\n")

  } else {
    log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
  }

  flag.Usage = func() {
    fmt.Fprintf(os.Stderr, "Usage: webtty [options] <command> [args]\n\n")
    fmt.Fprintf(os.Stderr, "Options:\n")
    fmt.Fprintf(os.Stderr, "  -l <address>           Listen Address (default is 127.0.0.1)\n")
    fmt.Fprintf(os.Stderr, "  -p <port>              Listen Port (default is 8080)\n")
    fmt.Fprintf(os.Stderr, "  -ft <dir>              File Transfer Directory\n")
    fmt.Fprintf(os.Stderr, "  -r <Nh|Nd|Nw|Nm|Ny>    File Retention Period\n")
    fmt.Fprintf(os.Stderr, "  -weblog                Enable WebLog via \"/logs\"\n")
    fmt.Fprintf(os.Stderr, "  -xff                   Use X-Forwarded-For in Logs\n\n")
    fmt.Fprintf(os.Stderr, "Environment Variables:\n")
    fmt.Fprintf(os.Stderr, "  WEBTTY_WEBLOG_TOKEN    Auth Token for WebLog\n")
    fmt.Fprintf(os.Stderr, "  WEBTTY_PASSWORD        Password for WebTTY\n\n")
  }

  var ftRetPeriod time.Duration
  lPtr := flag.String("l", "127.0.0.1", "")
  pPtr := flag.Int("p", 8080, "")
  xffPtr := flag.Bool("xff", false, "")
  webLogPtr := flag.Bool("weblog", false, "")
  ftDirPtr := flag.String("ft", "", "")
  ftRetPtr := flag.String("r", "", "")
  flag.Parse()

  if (len(flag.Args()) == 0) || ((len(*ftRetPtr) > 0) && (len(*ftDirPtr) == 0)) {
    flag.Usage()
    os.Exit(1);
  }

  if len(*ftRetPtr) > 0 {
    if m := regexp.MustCompile(`^(?i)([1-9][0-9]*)([hdwmy])$`).FindStringSubmatch(*ftRetPtr); m != nil {
      i, _ := strconv.Atoi(m[1])

      switch strings.ToLower(m[2]) {
      case "h":
        ftRetPeriod = time.Duration(i) * time.Hour
      case "d":
        ftRetPeriod = time.Duration(i) * time.Hour * 24
      case "w":
        ftRetPeriod = time.Duration(i) * time.Hour * 24 * 7
      case "m":
        ftRetPeriod = time.Duration(i) * time.Hour * 24 * 30
      case "y":
        ftRetPeriod = time.Duration(i) * time.Hour * 24 * 365
      }
    } else {
      log.Fatalf("Error: Invalid format for argument -r: must match regexp \"^[0-9][hdwmy]$\"\n")
    }
  }

  sCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer stop()

  mux := http.NewServeMux()
  subFS, _ := fs.Sub(www, "www")
  if tmpl, err := template.ParseFS(subFS, "*.html"); err == nil {
    if len(*ftDirPtr) > 0 {
      if dn, err := filepath.Abs(*ftDirPtr); err == nil {
        if tf, err := os.CreateTemp(dn, ""); err == nil {
          tf.Close(); os.Remove(tf.Name())
          mux.Handle("GET /", wwwHandler(http.FileServer(http.FS(subFS)), tmpl, Version, true))
          mux.Handle("GET /ft", wwwHandler(http.FileServer(http.FS(subFS)), tmpl, Version, true))
          mux.HandleFunc("GET /ft/", ftHandler(dn))
          mux.HandleFunc("PUT /ft/", ftHandler(dn))

          if ftRetPeriod > 0 {
            go func() {
              r := regexp.MustCompile(`^(?i)_[A-Z2-9]{22}$`)
              for {
                var expired []string
                n := time.Now()

                ftMutex.RLock()
                if entries, err := os.ReadDir(dn); err == nil {
                  for _, e := range entries {
                    if e.IsDir() && r.MatchString(e.Name()) {
                      if info, err := e.Info(); err == nil {
                        if n.Sub(info.ModTime()) > ftRetPeriod {
                          expired = append(expired, e.Name())
                        }
                      }
                    }
                  }
                }
                ftMutex.RUnlock()

                if len(expired) > 0 {
                  ftMutex.Lock()
                  for _, e := range expired {
                    if err := os.RemoveAll(dn + "/" + e); err == nil {
                      slog("{\033[32mOK\033[0m} Deleted %s/%s - Retention Period Expired", dn, e)

                    } else {
                      slog("{\033[31mERR\033[0m} Unable to Delete %s/%s - %v", dn, e, err)
                    }
                  }
                  ftMutex.Unlock()
                }
                time.Sleep(time.Minute)
              }
            }()
          }
        } else {
          log.Fatalf("Error: Unable to write to file transfer directory \"%s\"\n", dn)
        }
      } else {
        log.Fatalf("Error: %v\n", err)
      }
    } else {
      mux.Handle("GET /", wwwHandler(http.FileServer(http.FS(subFS)), tmpl, Version, false))
      mux.HandleFunc("GET /ft/{$}", http.NotFound)
      mux.HandleFunc("GET /ft.html", http.NotFound)
    }

    mux.HandleFunc("GET /_webtty", webTtyHandler(flag.Args()))

    if *webLogPtr {
      if webLogToken, defined := os.LookupEnv("WEBTTY_WEBLOG_TOKEN"); defined {
        mux.HandleFunc("GET /_logs", logHandler(webLogToken))

      } else {
        fmt.Fprintf(os.Stdout, "Error: Environment WEBTTY_WEBLOG_TOKEN is not defined\n")
        os.Exit(1)
      }
    } else {
      mux.HandleFunc("GET /logs", http.NotFound)
      mux.HandleFunc("GET /logs.html", http.NotFound)
    }

    s := &http.Server {
      Addr: fmt.Sprintf("%s:%d", *lPtr, *pPtr),
      Handler: logRequest(mux, *xffPtr),
      BaseContext: func(net.Listener) context.Context {
        return sCtx
      },
    }

    go func() {
      slog("Starting WebTTY (PID is %d) on http://%v...\n", os.Getpid(), s.Addr)

      if err := s.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("Error: %v\n", err)
      }
    }()

    <-sCtx.Done()
    slog("Caught Signal... Terminating...\n")
    cCtx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
    defer cancel()

    s.Shutdown(cCtx)

  } else {
    fmt.Fprintf(os.Stdout, "Error: %v\n", err)
    os.Exit(1)
  }
}
