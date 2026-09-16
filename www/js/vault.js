(() => {
  let active = null;
  let timeout = 5000;
  let sdata = null;

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
      if (active == 'admin') {
        fetch('/v1/users', { signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 200) {
            r.json().then((users) => {
              let u = Object.keys(users).sort((a, b) => {
                if (a === 'root') {
                  return -1;
                }
                return a[1] - b[1];
              }).reduce((a, c) => (a[c] = users[c], a), {});

              content.innerHTML = '<pre><h6 class="fw-bold">Vault Users</h6>' + quote(JSON.stringify(u, null, 2)) + '</pre>';
            });

          } else {
            window.location.reload();
          }
        });

      } else {
        fetch('/v1/namespaces', { signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 200) {
            r.json().then(async(namespaces) => {
              let data = {};
              sdata = {};

              for (let ns in namespaces) {
                let r = await fetch('/v1/data/' + ns, { signal: AbortSignal.timeout(timeout) });
                if (r.status === 200) {
                  let namespace = await r.json();

                  data[ns] = {};
                  sdata[ns] = namespace;

                  for (let key in namespace) {
                    data[ns][key] = {
                      'data': '<span class=data data-ns=' + ns + ' data-var=' + key + '>*****</span>'
                    };
                  }

                } else {
                  window.location.reload();
                }
              }
              content.innerHTML = '<pre><h6 class="fw-bold">Namespace Variables</h6>' + JSON.stringify(data, null, 2) + '</pre>';

              document.querySelectorAll('span[data-var]:not([data-var=""])').forEach((el) => {
                el.addEventListener('click', (e) => {
                  e.preventDefault();
                  document.getElementById('data-var').innerHTML = e.target.getAttribute('data-ns') + ' / ' + e.target.getAttribute('data-var');
                  document.getElementById('data-value').innerHTML = quote(JSON.stringify(sdata[e.target.getAttribute('data-ns')][e.target.getAttribute('data-var')], null, 2));
                  new bootstrap.Modal(document.getElementById('data')).show();
                });
              });
            });

          } else {
            window.location.reload();
          }
        });
      }
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
        fetch('/v1/logout', { method: 'POST', signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 204) {
            window.location.href = '/login.html';

          } else {
            setStatus(r.statusText);
          }
        });

      } catch (error) {
        setStatus(error);
      }
    });

    try {
      fetch('/v1/whoami', { signal: AbortSignal.timeout(timeout) }).then((r) => {
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
          setStatus(r.statusText);
        }
      });

    } catch (error) {
      setStatus(error);
    }
  });
})();
