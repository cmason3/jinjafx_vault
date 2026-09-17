(() => {
  let active = null;
  let timeout = 5000;

  function setStatus(message) {
    s = document.getElementById('status');
    s.innerHTML = message;
    new bootstrap.Modal(document.getElementById('error')).show();
  }

  function quote(str) {
    str = str.replace(/&/g, "&amp;");
    str = str.replace(/>/g, "&gt;");
    str = str.replace(/</g, "&lt;");
    str = str.replace(/"/g, "&quot;");
    str = str.replace(/'/g, "&apos;");
    return str;
  }

  function buildPage() {
    let content = document.getElementById('content');
    content.innerHTML = '';

    try {
      let innerHTML = '';

      fetch('v1/namespaces', { signal: AbortSignal.timeout(timeout) }).then((r) => {
        if (r.status === 200) {
          r.json().then(async(namespaces) => {
            if (active === 'admin') {
              fetch('v1/users', { signal: AbortSignal.timeout(timeout) }).then((r) => {
                if (r.status === 200) {
                  r.json().then((users) => {
                    let u = Object.keys(users).sort((a, b) => {
                      if (a === 'root') {
                        return -1;
                      }
                      return a[1] - b[1];
                    }).reduce((a, c) => (a['<span class=key>' + c + '</span>'] = users[c], a), {});

                    innerHTML += '<div style="display: flex; gap: 30px;">';
                    innerHTML += '<div style="width: 100%;">';
                    innerHTML += '<h5 class="pt-2 pb-3">Vault Users</h5>';
                    innerHTML += '<pre class="ps-3">' + JSON.stringify(u, null, 2) + '</pre></div>';
                    innerHTML += '<div style="width: 100%;">';
                    innerHTML += '<h5 class="pt-2 pb-3">Vault Namespaces</h5>';
                    innerHTML += '<pre class="ps-3">' + JSON.stringify(Object.keys(namespaces).sort(), null, 2) + '</pre></div>';
                    content.innerHTML = innerHTML + '</div>';
                  });
      
                } else {
                  window.location.reload();
                }
              });

            } else {
              let data = {};

              for (let ns in namespaces) {
                let r = await fetch('v1/data/' + ns, { signal: AbortSignal.timeout(timeout) });
                if (r.status === 200) {
                  let namespace = await r.json();
                  data[ns] = namespace

                } else {
                  window.location.reload();
                }
              }

              innerHTML += '<h5 class="pt-2 pb-3">Namespace Variables</h5>';
              innerHTML += '<div class="ps-3 pe-3" style="display: flex; gap: 30px;">';

              for (let ns of Object.keys(data).sort()) {
                innerHTML += '<div style="width: 100%;"><h5 class="pb-1 text-danger">' + ns + '</h5><ul class="list-group">';

                for (let key of Object.keys(data[ns]).sort()) {
                  innerHTML += '<li class="list-group-item"><span class=data data-ns=' + ns + ' data-var=' + key + '>' + key + '</span></li>'
                }
                innerHTML += '</ul></div>'
              }
              content.innerHTML = innerHTML + '</div>';

              document.querySelectorAll('span[data-var]:not([data-var=""])').forEach((el) => {
                el.addEventListener('click', (e) => {
                  e.preventDefault();
                  document.getElementById('data-var').innerHTML = e.target.getAttribute('data-ns') + ' / ' + e.target.getAttribute('data-var');
                  document.getElementById('data-value').innerHTML = quote(JSON.stringify(data[e.target.getAttribute('data-ns')][e.target.getAttribute('data-var')], null, 2));
                  new bootstrap.Modal(document.getElementById('data')).show();
                });
              });
            }
          });
        } else {
          window.location.reload();
        }
      });




      /*
      if (active == 'admin') {

      } else {
        fetch('v1/namespaces', { signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 200) {
            r.json().then(async(namespaces) => {






            });

          } else {
            window.location.reload();
          }
        });
      }
      */

    } catch (error) {
      content.innerHTML = '';
      setStatus(error);
    }
  }

  window.addEventListener('load', (e) => {
    document.getElementById('admin').addEventListener('click', (e) => {
      if (active != 'admin') {
        document.getElementById('user').classList.remove('active');
        document.getElementById('admin').classList.add('active');
        active = 'admin';
        buildPage();
      }
    });

    document.getElementById('user').addEventListener('click', (e) => {
      if (active != 'user') {
        document.getElementById('admin').classList.remove('active');
        document.getElementById('user').classList.add('active');
        active = 'user';
        buildPage();
      }
    });

    document.getElementById('logout').addEventListener('click', (e) => {
      try {
        fetch('v1/logout', { method: 'POST', signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 204) {
            window.location.href = 'login.html';

          } else {
            r.text().then((msg) => {
              setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
            });
          }
        });

      } catch (error) {
        setStatus(error);
      }
    });

    try {
      fetch('v1/whoami', { signal: AbortSignal.timeout(timeout) }).then((r) => {
        if (r.status === 200) {
          r.json().then((obj) => {
            if (obj.roles.includes('admin')) {
              document.getElementById('admin').classList.add('active');
              active = 'admin';

              if (!obj.roles.includes('user')) {
                document.getElementById('user').classList.add('disabled');
              }
            } else {
              document.getElementById('admin').classList.add('disabled');
              document.getElementById('user').classList.add('active');
              active = 'user';
            }
            buildPage();
          });

        } else {
          r.text().then((msg) => {
            setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
          });
        }
      });

    } catch (error) {
      setStatus(error);
    }
  });
})();
