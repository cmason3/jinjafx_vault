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
  "fmt"
  "net"
  "log"
  "time"
  "flag"
  "sync"
  "bytes"
  "slices"
  "regexp"
  "context"
  "strings"
  "strconv"
  "syscall"
  "net/http"
  "os/signal"
  "crypto/tls"
  "crypto/rand"
  "crypto/cipher"
  "encoding/json"
  "encoding/binary"
  "golang.org/x/term"
  "golang.org/x/crypto/argon2"
  "golang.org/x/crypto/chacha20poly1305"
  "github.com/akamensky/base58"
  "github.com/go-ldap/ldap/v3"
  "github.com/google/uuid"
)

const Version = "0.2.1"

var args struct {
  listen, tlsCrt, tlsKey string
  tls, xff, insecure bool
  idle time.Duration
  rlimit []int
  port int
}

type user struct {
  Roles []string `json:"roles"`
  Namespaces map[string]string `json:"namespaces,omitzero"`
  Password string `json:"password,omitempty"`
  LastChanged time.Time `json:"last_changed,omitzero"`
  Expiry string `json:"expiry,omitempty"`
  LdapServer string `json:"ldap_server,omitempty"`
  LdapDomain string `json:"ldap_domain,omitempty"`
  Disabled bool `json:"disabled"`
}
type data struct {
  Data any `json:"data"`
}
type audit struct {
  User string `json:"user"`
  Address string `json:"address"`
  Message string `json:"message"`
}
var vault struct {
  Users map[string]*user `json:"users"`
  Namespaces map[string]map[string]*data `json:"namespaces"`
  Audit map[time.Time]*audit `json:"audit"`
}

type authToken struct {
  user string
  expires time.Time
}
var authTokens = make(map[string]*authToken)
var userRateLimits = make(map[string][]time.Time)

var rUser = `[a-z][a-z0-9_]*`
var rNamespace = `[a-z][a-z0-9_:-]*`
var rVariable = `[a-z][a-z0-9_]*`

var vaultFile string
var vaultCipher cipher.AEAD
var vaultMutex sync.RWMutex
var authMutex sync.RWMutex
var rateMutex sync.Mutex

type httpWriter struct {
  http.ResponseWriter
  remoteHost, remoteUser string
  statusCode int
}
func responseWriter(w http.ResponseWriter) *httpWriter {
  return &httpWriter { w, "", "", http.StatusOK }
}
func (w *httpWriter) WriteHeader(statusCode int) {
  w.statusCode = statusCode
  w.ResponseWriter.WriteHeader(w.statusCode)
}

func slog(f string, a ...any) {
  m := fmt.Sprintf(f, a...)
  if _, defined := os.LookupEnv("JOURNAL_STREAM"); defined {
    m = regexp.MustCompile(`\033\[(?:(?:[01];)?[0-9][0-9]|0)m`).ReplaceAllString(m, "")
  }
  log.Print(m)
}

func ternary[T any](c bool, t, f T) T {
  if c {
    return t
  }
  return f
}

func logRequest(h http.Handler) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    _w := responseWriter(w)

    if args.xff && r.Header.Get("X-Forwarded-For") != "" {
      _w.remoteHost = r.Header.Get("X-Forwarded-For")

    } else {
      remoteHost, _, _ := net.SplitHostPort(r.RemoteAddr)
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

      remoteUser := ternary(len(_w.remoteUser) > 0, fmt.Sprintf(" [\033[34m%s\033[0m]", _w.remoteUser), "")
      slog("[%s] {%s} %s %s%s\n", _w.remoteHost, statusCode, r.Method, r.URL.Path, remoteUser)
    }
  }
}

func isAuthenticated(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
  authMutex.RLock()
  defer authMutex.RUnlock()

  t := r.Header.Get("X-Vault-Token")
  if v, ok := authTokens[t]; ok {
    if _, ok := vault.Users[v.user]; ok && !vault.Users[v.user].Disabled && (time.Now().UTC().Before(v.expires)) {
      w.(*httpWriter).remoteUser = v.user

      go func(t string) {
        authMutex.Lock()
        defer authMutex.Unlock()

        if _, ok := authTokens[t]; ok {
          authTokens[t].expires = time.Now().UTC().Add(args.idle)
        }
      }(t)

      if len(vault.Users[v.user].Password) > 0 && vault.Users[v.user].LastChanged.IsZero() {
        return v.user, true, true
      }

      if ln := len(vault.Users[v.user].Expiry); ln > 0 {
        var d time.Duration
        n, _ := strconv.Atoi(vault.Users[v.user].Expiry[ln-2:])
  
        switch vault.Users[v.user].Expiry[:ln-2] {
          case "hr":
            d = time.Duration(n) * time.Hour
          case "dy":
            d = time.Duration(n) * time.Hour * 24
          case "wk":
            d = time.Duration(n) * time.Hour * 24 * 7
          case "mh":
            d = time.Duration(n) * time.Hour * 24 * 30
          case "yr":
            d = time.Duration(n) * time.Hour * 24 * 365
        }
        
        if time.Now().UTC().After(vault.Users[v.user].LastChanged.Add(d)) {
          return v.user, true, true
        }
      }
      return v.user, true, false
    }
  }
  return "", false, false
}

func readVaultFile(env bool) error {
  var vaultKey []byte
  var dflag bool

  if b, err := os.ReadFile(vaultFile); err == nil {
    if env {
      if k, defined := os.LookupEnv("JFX_VAULT_KEY"); defined {
        if vaultKey, err = base58.Decode(k); err != nil {
          return err
        }
      }
    }

    if len(vaultKey) == 0 {
      fmt.Fprintf(os.Stderr, "Vault Key: ")
      k, _ := term.ReadPassword(int(os.Stdin.Fd()))
      fmt.Fprintf(os.Stderr, "\n")
      dflag = true

      if vaultKey, err = base58.Decode(string(k)); err != nil {
        return err
      }
    }

    if vaultCipher, err = chacha20poly1305.NewX(vaultKey); err != nil {
      return err
    }

    if plaintext, err := vaultCipher.Open(nil, b[1:chacha20poly1305.NonceSizeX + 1], b[chacha20poly1305.NonceSizeX + 1:], nil); err == nil {
      if err := json.Unmarshal(plaintext, &vault); err != nil {
        return err
      }
    } else {
      return err
    }
  } else {
    return err
  }
  if dflag {
    fmt.Fprintf(os.Stderr, "\n")
  }
  return nil
}

func writeVaultFile(init bool) error {
  flags := ternary(init, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.O_WRONLY)
  nonce := make([]byte, chacha20poly1305.NonceSizeX)

  if b, err := json.Marshal(vault); err == nil {
    if _, err := rand.Read(nonce); err == nil {
      ciphertext := vaultCipher.Seal(nil, nonce, b, nil)
      ciphertext = slices.Concat([]byte{ 1 }, nonce, ciphertext)

      if f, err := os.OpenFile(vaultFile, flags, 0600); err == nil {
        defer f.Close()
        f.Write(ciphertext)

      } else {
        return err
      }
    }
  } else {
    return err
  }
  return nil
}

func isPasswordValid(password string, old_password string) (bool, error) {
  e := fmt.Errorf("password complexity mismatch - length >= 12; upper >= 1; lower >= 1; numeric >= 1; special >= 1; differences >= 4")

  if len(password) >= 12 {
    if len(regexp.MustCompile(`[A-Z]`).FindAllStringIndex(password, -1)) < 1 {
      return false, e

    } else if len(regexp.MustCompile(`[a-z]`).FindAllStringIndex(password, -1)) < 1 {
      return false, e

    } else if len(regexp.MustCompile(`[0-9]`).FindAllStringIndex(password, -1)) < 1 {
      return false, e

    } else if m := len(regexp.MustCompile(`[ !#$%&()*+,-./:;<=>?@][^_{|}~]`).FindAllStringIndex(password, -1)); m < 1 {
      return false, e

    } else {
      var differences int

      for _, r := range password {
        if !strings.ContainsRune(old_password, r) {
          differences += 1
        }
      }
      if differences < 4 {
        return false, e
      }
    }
    return true, nil
  }
  return false, e
}

func newRootPassword() (string, error) {
  fmt.Fprintf(os.Stderr, "Root Password: ")
  p, _ := term.ReadPassword(int(os.Stdin.Fd()))
  fmt.Fprintf(os.Stderr, "\n")

  if ok, err := isPasswordValid(string(p), ""); ok {
    fmt.Fprintf(os.Stderr, "Verify Password: ")
    v, _ := term.ReadPassword(int(os.Stdin.Fd()))
    fmt.Fprintf(os.Stderr, "\n")

    if bytes.Compare(p, v) == 0 {
      fmt.Fprintf(os.Stderr, "\n")
      return string(p), nil

    } else {
      return "", fmt.Errorf("password verification failed")
    }
  } else {
    return "", err
  }
}

func verifyPassword(password string, hash string) bool {
  if key, err := base58.Decode(hash); err == nil {
    if (len(key) == 58) && (key[0] == 1) {
      t := binary.BigEndian.Uint32(key[1:1 + 4])
      m := binary.BigEndian.Uint32(key[1 + 4:1 + 4 + 4])
      p := uint8(key[1 + 4 + 4])
      salt := key[1 + 4 + 4 + 1:1 + 4 + 4 + 1 + 16]
      h := argon2.IDKey([]byte(password), salt, t, m, p, 32)
      return bytes.Compare(h, key[1 + 4 + 4 + 1 + 16:]) == 0
    }
  }
  return false
}

func getPasswordHash(password string) (string, error) {
  v, t, m, p := uint8(1), uint32(8), uint32(32 * 1024), uint8(4)
  salt := make([]byte, 16)
  key := make([]byte, 1 + 4 + 4 + 1 + len(salt) + 32)

  if _, err := rand.Read(salt); err != nil {
    return "", err
  }
  key[0], key[1 + 4 + 4] = v, p
  binary.BigEndian.PutUint32(key[1:1 + 4], t)
  binary.BigEndian.PutUint32(key[1 + 4:1 + 4 + 4], m)
  copy(key[1 + 4 + 4 + 1:], salt)
  copy(key[1 + 4 + 4 + 1 + len(salt):], argon2.IDKey([]byte(password), salt, t, m, p, 32))
  return base58.Encode(key), nil
}

func defaultHandler() http.HandlerFunc {
  return func(w http.ResponseWriter, _ *http.Request) {
    http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
  }
}

func apiHandler() http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
      apiGetHandler(w, r)

    } else if r.Method == http.MethodPost {
      if r.URL.Path == "/login" {
        apiLoginHandler(w, r)

      } else if r.URL.Path == "/logout" {
        apiLogoutHandler(w, r)

      } else {
        apiPostHandler(w, r)
      }
    } else if r.Method == http.MethodDelete {
      apiDeleteHandler(w, r)

    } else {
      http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
    }
  }
}

func apiLoginHandler(w http.ResponseWriter, r *http.Request) {
  var request struct {
    User string `json:"user"`
    Password string `json:"password"`
  }

  if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
    u := strings.ToLower(request.User)
    w.(*httpWriter).remoteUser = u

    vaultMutex.RLock()
    defer vaultMutex.RUnlock()

    if v, ok := vault.Users[u]; ok {
      if !vault.Users[u].Disabled {
        var i int

        key := fmt.Sprintf("%s/%s", w.(*httpWriter).remoteHost, u)
        now := time.Now().UTC()

        rateMutex.Lock()
        defer rateMutex.Unlock()

        if _, exists := userRateLimits[key]; !exists {
          userRateLimits[key] = make([]time.Time, 0)

        } else {
          cutoff := now.Add(-(time.Duration(args.rlimit[1]) * time.Minute))

          for ; i < len(userRateLimits[key]); i++ {
            if userRateLimits[key][i].After(cutoff) {
              userRateLimits[key] = userRateLimits[key][i:]
              break
            }
          }
        }

        if len(userRateLimits[key]) < args.rlimit[0] {
          if len(vault.Users[u].Password) > 0 {
            if !verifyPassword(request.Password, v.Password) {
              userRateLimits[key] = append(userRateLimits[key], now)
              http.Error(w, "Password Verification Failed", http.StatusUnauthorized)
              return
            }
          } else if len(vault.Users[u].LdapServer) > 0 {
            ldap.DefaultTimeout = time.Second * 5
            if l, err := ldap.DialURL(fmt.Sprintf("ldaps://%s", vault.Users[u].LdapServer), ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: args.insecure})); err == nil {
              defer l.Close()
    
              if err := l.Bind(fmt.Sprintf("%s\\%s", vault.Users[u].LdapDomain, u), request.Password); err != nil {
                userRateLimits[key] = append(userRateLimits[key], now)
                http.Error(w, err.Error(), http.StatusUnauthorized)
                return
              }
            } else {
              userRateLimits[key] = append(userRateLimits[key], now)
              http.Error(w, err.Error(), http.StatusUnauthorized)
              return
            }
          } else {
            http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
            return
          }
        } else {
          userRateLimits[key] = append(userRateLimits[key][1:], now)
          http.Error(w, fmt.Sprintf("Too Many Failed Attempts (%d in %dm)", args.rlimit[0], args.rlimit[1]), http.StatusTooManyRequests)
          return
        }
  
        authMutex.Lock()
        defer authMutex.Unlock()
  
        t := uuid.NewString()

        authTokens[t] = &authToken {
          user: u,
          expires: time.Now().UTC().Add(args.idle),
        }

        response := struct {
          Token string `json:"token"`
          Expires time.Time `json:"expires"`
        } {
          Token: t,
          Expires: authTokens[t].expires,
        }
  
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
  
        e := json.NewEncoder(w)
        e.SetIndent("", "  ")
        e.Encode(response)
  
        go func() {
          authMutex.Lock()
          defer authMutex.Unlock()
  
          for k, v := range authTokens {
            if time.Now().UTC().After(v.expires) {
              delete(authTokens, k)
            }
          }
        }()

      } else {
        http.Error(w, "User Disabled", http.StatusUnauthorized)
      }
    } else {
      http.Error(w, "Password Verification Failed", http.StatusUnauthorized)
    }
  } else {
    http.Error(w, err.Error(), http.StatusBadRequest)
  }
}

func apiLogoutHandler(w http.ResponseWriter, r *http.Request) {
  vaultMutex.RLock()
  defer vaultMutex.RUnlock()

  if _, ok, _ := isAuthenticated(w, r); ok {
    authMutex.Lock()
    defer authMutex.Unlock()

    delete(authTokens, r.Header.Get("X-Vault-Token"))
    w.WriteHeader(http.StatusNoContent)

  } else {
    http.Error(w, "Not Logged In", http.StatusUnauthorized)
  }
}

func apiGetHandler(w http.ResponseWriter, r *http.Request) {
  vaultMutex.RLock()
  defer vaultMutex.RUnlock()

  if ruser, ok, expired := isAuthenticated(w, r); ok {
    if !expired {
      if r.URL.Path == "/audit" { // Get Audit Log
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          w.Header().Set("Cache-Control", "no-store")
          w.Header().Set("Content-Type", "application/json")
          w.WriteHeader(http.StatusOK)
  
          e := json.NewEncoder(w)
          e.SetIndent("", "  ")
          e.Encode(vault.Audit)
  
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if r.URL.Path == "/users" { // Get Users
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          w.Header().Set("Content-Type", "application/json")
          w.WriteHeader(http.StatusOK)
  
          e := json.NewEncoder(w)
          e.SetIndent("", "  ")
          e.Encode(vault.Users)
  
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if r.URL.Path == "/namespaces" { // Get Namespaces
        var ns = make(map[string]string)
  
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
  
        e := json.NewEncoder(w)
        e.SetIndent("", "  ")
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          for k := range vault.Namespaces {
            if p, exists := vault.Users[ruser].Namespaces[k]; exists {
              ns[k] = p

            } else {
              ns[k] = "na"
            }
          }
        } else {
          ns = vault.Users[ruser].Namespaces
        }
        e.Encode(ns)

      } else if m := regexp.MustCompile(`^/data/(` + rNamespace + `)/(` + rVariable + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Get Namespace Variable
        ns := strings.ToLower(m[1])
        k := strings.ToLower(m[2])
  
        if _, ok := vault.Users[ruser].Namespaces[ns]; ok {
          if v, ok := vault.Namespaces[ns][k]; ok {
            w.Header().Set("Cache-Control", "no-store")
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
  
            e := json.NewEncoder(w)
            e.SetIndent("", "  ")
            e.Encode(v)
  
          } else {
            http.Error(w, "Variable Not Found", http.StatusNotFound)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/data/(` + rNamespace + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Get Namespace Variables
        ns := strings.ToLower(m[1])
  
        if _, ok := vault.Users[ruser].Namespaces[ns]; ok {
          w.Header().Set("Cache-Control", "no-store")
          w.Header().Set("Content-Type", "application/json")
          w.WriteHeader(http.StatusOK)
  
          e := json.NewEncoder(w)
          e.SetIndent("", "  ")
          e.Encode(vault.Namespaces[ns])
  
        } else {
          http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
        }
      } else {
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
      }
    } else {
      http.Error(w, "User Password Expired - Password Change Required", http.StatusTeapot)
    }
  } else {
    http.Error(w, "Not Logged In", http.StatusUnauthorized)
  }
}

func apiPostHandler(w http.ResponseWriter, r *http.Request) {
  vaultMutex.Lock()
  defer vaultMutex.Unlock()

  if ruser, ok, expired := isAuthenticated(w, r); ok {
    if m := regexp.MustCompile(`^/chpass$`).FindStringSubmatch(r.URL.Path); m != nil { // Change Password for UserPass User
      if len(vault.Users[ruser].Password) > 0 {
        var request struct {
          OldPassword string `json:"old_password"`
          Password string `json:"password"`
        }
  
        if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
          if verifyPassword(request.OldPassword, vault.Users[ruser].Password) {
            if ok, err := isPasswordValid(request.Password, request.OldPassword); ok {
              if h, err := getPasswordHash(request.Password); err == nil {
                vault.Users[ruser].Password = h
                vault.Users[ruser].LastChanged = time.Now().UTC()
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("User Changed Password"),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, err.Error(), http.StatusInternalServerError)
              }
            } else {
              http.Error(w, err.Error(), http.StatusBadRequest)
            }
          } else {
            http.Error(w, "Password Verification Failed", http.StatusUnauthorized)
          }
        } else {
          http.Error(w, err.Error(), http.StatusBadRequest)
        }
      } else {
        http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
      }
    } else if !expired {
      if m := regexp.MustCompile(`^/namespace/(` + rNamespace + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Create Namespace
        ns := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if _, ok := vault.Namespaces[ns]; !ok {
            vault.Namespaces[ns] = make(map[string]*data)
  
            vault.Audit[time.Now().UTC()] = &audit {
              User: ruser,
              Address: w.(*httpWriter).remoteHost,
              Message: fmt.Sprintf("Created Namespace '%s'", ns),
            }
  
            if err := writeVaultFile(false); err == nil {
              w.WriteHeader(http.StatusCreated)
  
            } else {
              http.Error(w, err.Error(), http.StatusInternalServerError)
            }
          } else {
            http.Error(w, "Namespace Already Exists", http.StatusConflict)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/userpass$`).FindStringSubmatch(r.URL.Path); m != nil { // Create or Update UserPass User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          var request struct {
            Password string `json:"password"`
          }
  
          if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
            if len(request.Password) > 0 {
              if ok, err := isPasswordValid(request.Password, ""); ok {
                if h, err := getPasswordHash(request.Password); err == nil {
                  var cflag int
  
                  if _, ok := vault.Users[u]; !ok {
                    vault.Users[u] = &user {
                      Roles: []string{"user"},
                      Password: h,
                      LastChanged: time.Now().UTC(),
                      Namespaces: make(map[string]string),
                    }
  
                    cflag = 1
  
                  } else if len(vault.Users[u].Password) > 0 {
                    if !verifyPassword(request.Password, vault.Users[u].Password) {
                      vault.Users[u].Password = h
                      vault.Users[u].LastChanged = time.Now().UTC()
                      cflag = 2
                    }
                  } else {
                    http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
                    return
                  }
  
                  if cflag > 0 {
                    vault.Audit[time.Now().UTC()] = &audit {
                      User: ruser,
                      Address: w.(*httpWriter).remoteHost,
                      Message: fmt.Sprintf("%s UserPass User '%s'", ternary(cflag == 1, "Created", "Updated"), u),
                    }
  
                    if err := writeVaultFile(false); err == nil {
                      w.WriteHeader(ternary(cflag == 1, http.StatusCreated, http.StatusNoContent))
  
                    } else {
                      http.Error(w, err.Error(), http.StatusInternalServerError)
                    }
                  } else {
                    http.Error(w, "User Not Modified", http.StatusNotModified)
                  }
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, err.Error(), http.StatusBadRequest)
              }
            } else {
              http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
            }
          } else {
            http.Error(w, err.Error(), http.StatusBadRequest)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/ldaps$`).FindStringSubmatch(r.URL.Path); m != nil { // Create or Update LDAP User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          var request struct {
            LdapServer string `json:"ldap_server"`
            LdapDomain string `json:"ldap_domain"`
          }
  
          if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
            if (len(request.LdapServer) > 0) && (len(request.LdapDomain) > 0) {
              var cflag int
  
              if _, ok := vault.Users[u]; !ok {
                vault.Users[u] = &user {
                  Roles: []string{"user"},
                  LdapServer: request.LdapServer,
                  LdapDomain: request.LdapDomain,
                  Namespaces: make(map[string]string),
                }
  
                cflag = 1
  
              } else if len(vault.Users[u].LdapServer) > 0 {
                if vault.Users[u].LdapServer != request.LdapServer {
                  vault.Users[u].LdapServer = request.LdapServer
                  cflag = 2
                }
  
                if vault.Users[u].LdapDomain != request.LdapDomain {
                 vault.Users[u].LdapDomain = request.LdapDomain
                  cflag = 2
                }
              } else {
                http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
                return
              }
  
              if cflag > 0 {
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("%s LDAP User '%s'", ternary(cflag == 1, "Created", "Updated"), u),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(ternary(cflag == 1, http.StatusCreated, http.StatusNoContent))
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
            }
          } else {
            http.Error(w, err.Error(), http.StatusBadRequest)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/chage/([1-9][0-9]*)(hr|dy|wk|mh|yr)$`).FindStringSubmatch(r.URL.Path); m != nil { // Set Expiration for UserPass User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if _, ok := vault.Users[u]; ok {
            if len(vault.Users[u].Password) > 0 {
              d := fmt.Sprintf("%s%s", m[2], m[3])

              if vault.Users[u].Expiry != d {
                vault.Users[u].Expiry = d
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Set Expiration for User '%s' to '%s'", u, d),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, "Not UserPass User", http.StatusBadRequest)
            }
          } else {
            http.Error(w, "User Not Found", http.StatusNotFound)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/expire$`).FindStringSubmatch(r.URL.Path); m != nil { // Force Password Change for UserPass User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if ruser != u {
            if _, ok := vault.Users[u]; ok {
              if len(vault.Users[u].Password) > 0 {
                if !vault.Users[u].LastChanged.IsZero() {
                  vault.Users[u].LastChanged = time.Time{}
  
                  vault.Audit[time.Now().UTC()] = &audit {
                    User: ruser,
                    Address: w.(*httpWriter).remoteHost,
                    Message: fmt.Sprintf("Forced Password Change for User '%s'", u),
                  }
  
                  if err := writeVaultFile(false); err == nil {
                    w.WriteHeader(http.StatusNoContent)
  
                  } else {
                    http.Error(w, err.Error(), http.StatusInternalServerError)
                  }
                } else {
                  http.Error(w, "User Not Modified", http.StatusNotModified)
                }
              } else {
                http.Error(w, "Not UserPass User", http.StatusBadRequest)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/disabled$`).FindStringSubmatch(r.URL.Path); m != nil { // Disable User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if ruser != u {
            if _, ok := vault.Users[u]; ok {
              if !vault.Users[u].Disabled {
                vault.Users[u].Disabled = true
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Disabled User '%s'", u),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/roles/(admin|user)$`).FindStringSubmatch(r.URL.Path); m != nil { // Assign Role to User
        u := strings.ToLower(m[1])
        role := strings.ToLower(m[2])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if u != "root" && ruser != u {
            if _, ok := vault.Users[u]; ok {
              if !slices.Contains(vault.Users[u].Roles, role) {
                vault.Users[u].Roles = append(vault.Users[u].Roles, role)
    
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Assigned Role '%s' to User '%s'", role, u),
                }
    
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
    
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/namespaces/(` + rNamespace + `)/(r[ow])$`).FindStringSubmatch(r.URL.Path); m != nil { // Assign User to Namespace
        u := strings.ToLower(m[1])
        ns := strings.ToLower(m[2])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if u != "root" {
            if _, ok := vault.Users[u]; ok {
              if _, ok := vault.Namespaces[ns]; ok {
                if p, ok := vault.Users[u].Namespaces[ns]; !ok || (p != m[3]) {
                  vault.Users[u].Namespaces[ns] = m[3]
  
                  vault.Audit[time.Now().UTC()] = &audit {
                    User: ruser,
                    Address: w.(*httpWriter).remoteHost,
                    Message: fmt.Sprintf("Assigned User '%s' to Namespace '%s' as '%s'", u, ns, m[3]),
                  }
  
                  if err := writeVaultFile(false); err == nil {
                    w.WriteHeader(http.StatusNoContent)
  
                  } else {
                    http.Error(w, err.Error(), http.StatusInternalServerError)
                  }
                } else {
                  http.Error(w, "User Not Modified", http.StatusNotModified)
                }
              } else {
                http.Error(w, "Namespace Not Found", http.StatusNotFound)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/data/(` + rNamespace + `)/(` + rVariable + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Create or Update Namespace Variable
        ns := strings.ToLower(m[1])
        k := strings.ToLower(m[2])
  
        if v, ok := vault.Users[ruser].Namespaces[ns]; ok && v == "rw" {
          var request data
  
          if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
            vault.Namespaces[ns][k] = &request
  
            if err := writeVaultFile(false); err == nil {
              w.WriteHeader(http.StatusNoContent)
  
            } else {
              http.Error(w, err.Error(), http.StatusInternalServerError)
            }
          } else {
            http.Error(w, err.Error(), http.StatusBadRequest)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/data/(` + rNamespace + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Create or Update Namespace Variables
        ns := strings.ToLower(m[1])
  
        if v, ok := vault.Users[ruser].Namespaces[ns]; ok && v == "rw" {
          var request map[string]*data
  
          if err := json.NewDecoder(r.Body).Decode(&request); err == nil {
            for k := range request {
              if !regexp.MustCompile(`^` + rVariable + `$`).MatchString(k) {
                http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
                return
              }
            }
            for k := range request {
              vault.Namespaces[ns][strings.ToLower(k)] = &data {
                Data: request[k].Data,
              }
            }
  
            if err := writeVaultFile(false); err == nil {
              w.WriteHeader(http.StatusNoContent)
  
            } else {
              http.Error(w, err.Error(), http.StatusInternalServerError)
            }
          } else {
            http.Error(w, err.Error(), http.StatusBadRequest)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else {
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
      }
    } else {
      http.Error(w, "User Password Expired - Password Change Required", http.StatusTeapot)
    }
  } else {
    http.Error(w, "Not Logged In", http.StatusUnauthorized)
  }
}

func apiDeleteHandler(w http.ResponseWriter, r *http.Request) {
  vaultMutex.Lock()
  defer vaultMutex.Unlock()

  if ruser, ok, expired := isAuthenticated(w, r); ok {
    if !expired {
      if m := regexp.MustCompile(`^/namespace/(` + rNamespace + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Delete Namespace
        ns := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if _, ok := vault.Namespaces[ns]; ok {
            delete(vault.Namespaces, ns)
  
            for _, v := range vault.Users {
              delete(v.Namespaces, ns)
            }
  
            vault.Audit[time.Now().UTC()] = &audit {
              User: ruser,
              Address: w.(*httpWriter).remoteHost,
              Message: fmt.Sprintf("Deleted Namespace '%s'", ns),
            }
  
            if err := writeVaultFile(false); err == nil {
              w.WriteHeader(http.StatusNoContent)
  
            } else {
              http.Error(w, err.Error(), http.StatusInternalServerError)
            }
          } else {
            http.Error(w, "Namespace Not Found", http.StatusNotFound)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Delete User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if u != "root" && ruser != u {
            if _, ok := vault.Users[u]; ok {
              delete(vault.Users, u)
  
              vault.Audit[time.Now().UTC()] = &audit {
                User: ruser,
                Address: w.(*httpWriter).remoteHost,
                Message: fmt.Sprintf("Deleted User '%s'", u),
              }
  
              if err := writeVaultFile(false); err == nil {
                w.WriteHeader(http.StatusNoContent)
  
              } else {
                http.Error(w, err.Error(), http.StatusInternalServerError)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/chage$`).FindStringSubmatch(r.URL.Path); m != nil { // Clear Expiration for UserPass User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if _, ok := vault.Users[u]; ok {
            if len(vault.Users[u].Password) > 0 {
              if len(vault.Users[u].Expiry) > 0 {
                vault.Users[u].Expiry = ""
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Cleared Expiration for User '%s'", u),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, "Not UserPass User", http.StatusBadRequest)
            }
          } else {
            http.Error(w, "User Not Found", http.StatusNotFound)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/disabled$`).FindStringSubmatch(r.URL.Path); m != nil { // Enable User
        u := strings.ToLower(m[1])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if ruser != u {
            if _, ok := vault.Users[u]; ok {
              if vault.Users[u].Disabled {
                vault.Users[u].Disabled = false
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Enabled User '%s'", u),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "User Not Modified", http.StatusNotModified)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser +`)/roles/(admin|user)$`).FindStringSubmatch(r.URL.Path); m != nil { // Remove Role from User
        u := strings.ToLower(m[1])
        role := strings.ToLower(m[2])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if u != "root" && ruser != u {
            if _, ok := vault.Users[u]; ok {
              if slices.Contains(vault.Users[u].Roles, role) {
                vault.Users[u].Roles = slices.DeleteFunc(vault.Users[u].Roles, func(s string) bool {
                  return s == role
                })
    
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Removed Role '%s' from User '%s'", role, u),
                }
    
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
    
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "Role Not Found", http.StatusNotFound)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/user/(` + rUser + `)/namespaces/([\w:-]+)$`).FindStringSubmatch(r.URL.Path); m != nil { // Remove User from Namespace
        u := strings.ToLower(m[1])
        ns := strings.ToLower(m[2])
  
        if slices.Contains(vault.Users[ruser].Roles, "admin") {
          if u != "root" {
            if _, ok := vault.Users[u]; ok {
              if _, ok := vault.Namespaces[ns]; ok {
                delete(vault.Users[u].Namespaces, ns)
  
                vault.Audit[time.Now().UTC()] = &audit {
                  User: ruser,
                  Address: w.(*httpWriter).remoteHost,
                  Message: fmt.Sprintf("Removed User '%s' from Namespace '%s'", u, ns),
                }
  
                if err := writeVaultFile(false); err == nil {
                  w.WriteHeader(http.StatusNoContent)
  
                } else {
                  http.Error(w, err.Error(), http.StatusInternalServerError)
                }
              } else {
                http.Error(w, "Namespace Not Found", http.StatusNotFound)
              }
            } else {
              http.Error(w, "User Not Found", http.StatusNotFound)
            }
          } else {
            http.Error(w, "Permission Denied", http.StatusForbidden)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else if m := regexp.MustCompile(`^/data/(` + rNamespace + `)/(` + rVariable + `)$`).FindStringSubmatch(r.URL.Path); m != nil { // Delete Namespace Variable
        ns := strings.ToLower(m[1])
        k := strings.ToLower(m[2])
  
        if v, ok := vault.Users[ruser].Namespaces[ns]; ok && v == "rw" {
          if _, ok := vault.Namespaces[ns][k]; ok {
            delete(vault.Namespaces[ns], k)
  
            if err := writeVaultFile(false); err == nil {
              w.WriteHeader(http.StatusNoContent)
  
            } else {
              http.Error(w, err.Error(), http.StatusInternalServerError)
            }
          } else {
            http.Error(w, "Variable Not Found", http.StatusNotFound)
          }
        } else {
          http.Error(w, "Insufficient Privileges", http.StatusForbidden)
        }
      } else {
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
      }
    } else {
      http.Error(w, "User Password Expired - Password Change Required", http.StatusTeapot)
    }
  } else {
    http.Error(w, "Not Logged In", http.StatusUnauthorized)
  }
}

func main() {
  if _, defined := os.LookupEnv("JOURNAL_STREAM"); !defined {
    log.SetFlags(log.Flags() | log.Lmicroseconds)

    fmt.Fprintf(os.Stdout, "JinjaFx Vault v%s\n", Version)
    fmt.Fprintf(os.Stdout, "URL https://github.com/cmason3/jinjafx_vault\n\n")

  } else {
    log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
  }

  flag.Usage = func() {
    fmt.Fprintf(os.Stderr, "Usage: %s <action> [options]\n\n", os.Args[0])
    fmt.Fprintf(os.Stderr, "Actions:\n")
    fmt.Fprintf(os.Stderr, "  -i[nit] <jinjafx.vault>       Initialise New Vault\n")
    fmt.Fprintf(os.Stderr, "  -r[eset] <jinjafx.vault>      Reset Vault Credentials\n")
    fmt.Fprintf(os.Stderr, "  -s[erve] <jinjafx.vault>      Start Vault Server\n\n")
    fmt.Fprintf(os.Stderr, "Options:\n")
    fmt.Fprintf(os.Stderr, "  -l[isten] <address>           Listen Address (default is 127.0.0.1)\n")
    fmt.Fprintf(os.Stderr, "  -p[ort] <port>                Listen Port (default is http/8080 or https/8443)\n")
    fmt.Fprintf(os.Stderr, "  -idle <n(mn|hr)>              Change User Idle Timeout (default is 15mn)\n")
    fmt.Fprintf(os.Stderr, "  -rlimit <n/n(mn|hr)>          Change Login Rate Limit (default is 3/15mn)\n")
    fmt.Fprintf(os.Stderr, "  -tls                          Enable Transport Layer Security\n")
    fmt.Fprintf(os.Stderr, "   -tls.crt <vault.crt>         TLS Certificate Chain\n")
    fmt.Fprintf(os.Stderr, "   -tls.key <vault.key>         TLS Private Key\n")
    fmt.Fprintf(os.Stderr, "  -xff                          Use X-Forwarded-For in Logs\n")
    fmt.Fprintf(os.Stderr, "  -k                            Allow Insecure LDAPS\n\n")
    fmt.Fprintf(os.Stderr, "Environment Variables:\n")
    fmt.Fprintf(os.Stderr, "  JFX_VAULT_KEY                 JinjaFx Vault Key\n\n")
  }

  var action uint8
  actionFunc := func(n int) (func(s string) error) {
    return func(s string) error {
      action |= (1 << n); vaultFile = s;
      return ternary(strings.HasPrefix(s, "-"), fmt.Errorf(""), nil)
    }
  }

  flag.Func("i", "", actionFunc(0))
  flag.Func("init", "", actionFunc(0))
  flag.Func("r", "", actionFunc(1))
  flag.Func("reset", "", actionFunc(1))
  flag.Func("s", "", actionFunc(2))
  flag.Func("serve", "", actionFunc(2))
  flag.StringVar(&args.listen, "l", "127.0.0.1", "")
  flag.StringVar(&args.listen, "listen", "127.0.0.1", "")
  flag.IntVar(&args.port, "p", 0, "")
  flag.IntVar(&args.port, "port", 0, "")

  args.rlimit = []int{3, 15}
  flag.Func("rlimit", "", func(s string) error {
    if m := regexp.MustCompile(`^([1-9][0-9]*)/([1-9][0-9]*)(mn|hr)$`).FindStringSubmatch(s); m != nil {
      n1 , _ := strconv.Atoi(m[1])
      n2 , _ := strconv.Atoi(m[2])

      if m[3] == "hr" {
        n2 *= 60
      }
      args.rlimit = []int{n1, n2}
      return nil
    }
    return fmt.Errorf("")
  })

  args.idle = 15 * time.Minute
  flag.Func("idle", "", func(s string) error {
    if m := regexp.MustCompile(`^([1-9][0-9]*)(mn|hr)$`).FindStringSubmatch(s); m != nil {
      n , _ := strconv.Atoi(m[1])

      switch m[2] {
        case "mn":
          args.idle = time.Duration(n) * time.Minute
        case "hr":
          args.idle = time.Duration(n) * time.Hour
      }
      return nil
    }
    return fmt.Errorf("")
  })

  flag.BoolVar(&args.tls, "tls", false, "")
  flag.StringVar(&args.tlsCrt, "tls.crt", "", "")
  flag.StringVar(&args.tlsKey, "tls.key", "", "")
  flag.BoolVar(&args.xff, "xff", false, "")
  flag.BoolVar(&args.insecure, "k", false, "")
  flag.Parse()

  if (len(flag.Args()) > 0) || (action == 0) || (action & (action - 1) != 0) ||
   (args.tls && ((len(args.tlsCrt) == 0) || (len(args.tlsKey) == 0))) {
    flag.Usage()
    os.Exit(1);
  }

  if action == 1 { // -init
    vault.Users = make(map[string]*user)
    vault.Namespaces = make(map[string]map[string]*data)
    vault.Audit = make(map[time.Time]*audit)

    if p, err := newRootPassword(); err == nil {
      if h, err := getPasswordHash(p); err == nil {
        vault.Users["root"] = &user {
          Roles: []string{"admin"},
          Password: h,
          LastChanged: time.Now().UTC(),
        }
      } else {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
    } else {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }

    vault.Audit[time.Now().UTC()] = &audit {
      User: "system",
      Address: "local",
      Message: "Vault Initialised",
    }

    key := make([]byte, chacha20poly1305.KeySize)

    if _, err := rand.Read(key); err == nil {
      if vaultCipher, err = chacha20poly1305.NewX(key); err == nil {
        if err := writeVaultFile(true); err == nil {
          fmt.Fprintf(os.Stdout, "Vault Key is %s\n\n", base58.Encode(key))

        } else {
          fmt.Fprintf(os.Stderr, "Error: %v\n", err)
          os.Exit(1)
        }
      } else {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
    } else {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }
  } else if action == 2 { // -reset
    if err := readVaultFile(false); err != nil {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }

    if p, err := newRootPassword(); err == nil {
      if h, err := getPasswordHash(p); err == nil {
        vault.Users["root"].Password = h
        vault.Users["root"].LastChanged = time.Now().UTC()

      } else {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
    } else {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }

    vault.Audit[time.Now().UTC()] = &audit {
      User: "system",
      Address: "local",
      Message: "Vault Credentials Reset",
    }

    key := make([]byte, chacha20poly1305.KeySize)

    if _, err := rand.Read(key); err == nil {
      if vaultCipher, err = chacha20poly1305.NewX(key); err == nil {
        if err := writeVaultFile(false); err == nil {
          fmt.Fprintf(os.Stdout, "New Vault Key is %s\n\n", base58.Encode(key))

        } else {
          fmt.Fprintf(os.Stderr, "Error: %v\n", err)
          os.Exit(1)
        }
      } else {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
    } else {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }
  } else if action == 4 { // -serve
    if err := readVaultFile(true); err != nil {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }

    mux := http.NewServeMux()
    mux.Handle("/", defaultHandler())
    mux.Handle("/v1/", http.StripPrefix("/v1", apiHandler()))

    sCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    args.port = ternary(args.port == 0, ternary(args.tls, 8443, 8080), args.port)

    s := &http.Server {
      Addr: fmt.Sprintf("%s:%d", args.listen, args.port),
      Handler: logRequest(mux),
      ReadTimeout: time.Second * 15,
      WriteTimeout: time.Second * 30,
      IdleTimeout: time.Second * 120,
      BaseContext: func(net.Listener) context.Context {
        return sCtx
      },
    }

    proto := ternary(args.tls, "https", "http")
    slog("Starting JinjaFx Vault (PID is %d) on %s://%v...\n", os.Getpid(), proto, s.Addr)

    if args.tls {
      s.TLSConfig = &tls.Config {
        MinVersion: tls.VersionTLS12,
        MaxVersion: tls.VersionTLS13,
        CurvePreferences: []tls.CurveID {
          tls.X25519MLKEM768,
          tls.X25519,
          tls.CurveP256,
        },
        CipherSuites: []uint16 {
          tls.TLS_CHACHA20_POLY1305_SHA256,
          tls.TLS_AES_256_GCM_SHA384,
          tls.TLS_AES_128_GCM_SHA256,
          tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
          tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
          tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
          tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
          tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
          tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
          tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
          tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
        },
      }

      go func() {
        if err := s.ListenAndServeTLS(args.tlsCrt, args.tlsKey); err != http.ErrServerClosed {
          log.Fatalf("Error: %v\n", err)
        }
      }()

    } else {
      go func() {
        if err := s.ListenAndServe(); err != http.ErrServerClosed {
          log.Fatalf("Error: %v\n", err)
        }
      }()
    }

    <-sCtx.Done()
    slog("Caught Signal... Terminating...\n")
    cCtx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
    defer cancel()

    s.Shutdown(cCtx)
  }
}
