(() => {
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
            if (sessionStorage.getItem('active') === 'admin') {
              fetch('v1/users', { signal: AbortSignal.timeout(timeout) }).then((r) => {
                if (r.status === 200) {
                  r.json().then((users) => {
                    let u = Object.keys(users).sort((a, b) => {
                      if (a === 'root') {
                        return -1;
                      }
                      return a[1] - b[1];
                    }).reduce((a, c) => (a['<span class=key>' + c + '</span>'] = users[c], a), {});

                    innerHTML += '<div class="flex">';
                    innerHTML += '<div class="w-100">';
                    innerHTML += '<h5 class="pt-2 pb-3">Vault Users</h5>';
                    innerHTML += '<pre class="ms-3 me-3 p-1 border rounded">' + JSON.stringify(u, null, 2) + '</pre></div>';
                    innerHTML += '<div class="w-100">';
                    innerHTML += '<h5 class="pt-2 pb-3">Vault Namespaces</h5>';
                    innerHTML += '<pre class="ms-3 me-3 p-1 border rounded">' + JSON.stringify(Object.keys(namespaces).sort(), null, 2) + '</pre></div>';
                    content.innerHTML = innerHTML + '</div>';
                  });
      
                } else {
                  window.location.reload();
                }
              });

            } else {
              let data = {};

              for (let ns in namespaces) {
                if (namespaces[ns] !== 'na') {
                  let r = await fetch('v1/data/' + ns, { signal: AbortSignal.timeout(timeout) });
                  if (r.status === 200) {
                    let namespace = await r.json();
                    data[ns] = namespace

                  } else {
                    window.location.reload();
                  }
                }
              }

              innerHTML += '<h5 class="pt-2 pb-3">Namespace Variables</h5>';
              innerHTML += '<div class="ps-3 pe-3 flex">';

              if (Object.keys(data).length) {
                for (let ns of Object.keys(data).sort()) {
                  let p = namespaces[ns].replace('ro', 'read-only').replace('rw', 'read/write');
                  innerHTML += '<div class="w-100"><h5 class="pb-1 text-danger">' + ns + '<span class="text-secondary"> ' + p + '</span></h5><ul class="list-group">';

                  if (Object.keys(data[ns]).length) {
                    for (let key of Object.keys(data[ns]).sort()) {
                      innerHTML += '<li class="list-group-item"><span class=data data-ns=' + ns + ' data-var=' + key + '>' + key + '</span></li>'
                    }
                  } else {
                    innerHTML += '<li class="list-group-item">No Variables</span></li>'
                  }

                  innerHTML += '</ul></div>'
                }
              } else {
                innerHTML += '<div class="w-100"><h5 class="pb-1 text-danger">No Namespaces</h5>';
              }
              content.innerHTML = innerHTML + '</div>';

              document.querySelectorAll('span[data-var]:not([data-var=""])').forEach((el) => {
                el.addEventListener('click', (e) => {
                  e.preventDefault();
                  document.getElementById('data-var').innerHTML = '<span class="text-danger">' + e.target.getAttribute('data-ns') + '</span><span class="text-secondary"> ' + e.target.getAttribute('data-var') + '</span>';
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

    } catch (error) {
      content.innerHTML = '';
      setStatus(error);
    }
  }

  function buildApiModal(e) {
    let method = sessionStorage.getItem('api_method')
    if (!method) method = e.value;

    let url = sessionStorage.getItem(method + ':url')
    let request = sessionStorage.getItem(method + ':request')

    if (url) {
      document.getElementById('api_url').value = url;

    } else {
      document.getElementById('api_url').value = 'data/<namespace>/<variable>';
    }

    if (method === 'POST') {
      if (request) {
        document.getElementById('api_request').value = request;

      } else {
        document.getElementById('api_request').value = JSON.stringify({ 'data': '<value>' }, null, 2);
      }
      document.getElementById('api_request').readOnly = false;
      document.getElementById('api_request').disabled = false;

    } else if (method === 'GET') {
      document.getElementById('api_request').readOnly = true;
      document.getElementById('api_request').disabled = false;
      document.getElementById('api_request').value = '';

    } else {
      document.getElementById('api_request').disabled = true;
      document.getElementById('api_request').value = '';
    }
    document.getElementById('api_url').focus();
  }

  window.addEventListener('load', (e) => {
    document.getElementById('admin').addEventListener('click', (e) => {
      if (sessionStorage.getItem('active') !== 'admin') {
        document.getElementById('user').classList.remove('active');
        document.getElementById('admin').classList.add('active');
        sessionStorage.setItem('active', 'admin');
        buildPage();
      }
    });

    document.getElementById('user').addEventListener('click', (e) => {
      if (sessionStorage.getItem('active') != 'user') {
        document.getElementById('admin').classList.remove('active');
        document.getElementById('user').classList.add('active');
        sessionStorage.setItem('active', 'user');
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

    document.getElementById('api_modal').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('api_url').focus();
    });

    document.getElementById('api_modal').addEventListener('hidden.bs.modal', (e) => {
      document.getElementById('content').focus();
    });

    document.getElementById('api_method').addEventListener('change', (e) => {
      sessionStorage.setItem('api_method', e.target.value);
      buildApiModal(e.target);
    });

    document.getElementById('api_url').addEventListener('change', (e) => {
      let method = document.getElementById('api_method').value;
      sessionStorage.setItem(method + ':url', document.getElementById('api_url').value)
    });

    document.getElementById('api_request').addEventListener('change', (e) => {
      let method = document.getElementById('api_method').value;
      sessionStorage.setItem(method + ':request', document.getElementById('api_request').value)
    });

    document.getElementById('api_url').addEventListener('keyup', (e) => {
      if ((e.key === 'Enter') && (document.getElementById('api_url').value.trim().length !== 0)) {
        if (document.getElementById('api_method').value === 'POST') {
          document.getElementById('api_request').focus();

        } else {
          document.getElementById('api_submit').click();
        }
      }
    });

    document.getElementById('api_submit').addEventListener('click', (e) => {
      if (document.getElementById('api_url').value.trim().length === 0) {
        document.getElementById('api_url').focus();
        return;
      }

      try {
        if (document.getElementById('api_method').value === 'POST') {
          bootstrap.Modal.getInstance(document.getElementById('api_modal')).hide();
          body = document.getElementById('api_request').value;

          fetch('v1/' + document.getElementById('api_url').value, { method: 'POST', body: body, signal: AbortSignal.timeout(timeout) }).then((r) => {
            if ((r.status === 401) || (r.status === 418)) {
              window.location.reload();

            } else if (r.status >= 300) {
              r.text().then((msg) => {
                setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
              });

            } else {
              buildPage();
            }
          });

        } else if (document.getElementById('api_method').value === 'DELETE') {
          bootstrap.Modal.getInstance(document.getElementById('api_modal')).hide();
          fetch('v1/' + document.getElementById('api_url').value, { method: 'DELETE', signal: AbortSignal.timeout(timeout) }).then((r) => {
            if ((r.status === 401) || (r.status === 418)) {
              window.location.reload();

            } else if (r.status >= 300) {
              r.text().then((msg) => {
                setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
              });

            } else {
              buildPage();
            }
          });

        } else {
          fetch('v1/' + document.getElementById('api_url').value, { method: 'GET', signal: AbortSignal.timeout(timeout) }).then((r) => {
            if ((r.status === 401) || (r.status === 418)) {
              window.location.reload();

            } else if (r.status >= 300) {
              r.text().then((msg) => {
                bootstrap.Modal.getInstance(document.getElementById('api_modal')).hide();
                setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
              });

            } else {
              r.json().then((obj) => {
                document.getElementById('api_request').value = JSON.stringify(obj, null, 2);
              });
            }
          });
        }
      } catch (error) {
        setStatus(error);
      }
    });

    document.getElementById('api-help').addEventListener('click', (e) => {
      window.open('https://github.com/cmason3/jinjafx_vault#rest-api-methods');
    });

    document.getElementById('api').addEventListener('click', (e) => {
      buildApiModal(document.getElementById('api_method'));
      new bootstrap.Modal(document.getElementById('api_modal')).show();
    });

    try {
      fetch('v1/whoami', { signal: AbortSignal.timeout(timeout) }).then((r) => {
        if (r.status === 200) {
          r.json().then((obj) => {
            let active = sessionStorage.getItem('active');

            if ((active !== null) && !obj.roles.includes(active)) {
              active = null;
            }

            if (active === null) {
              sessionStorage.setItem('active', obj.roles.includes('admin') ? 'admin' : 'user');
            }

            for (let role of ['admin', 'user']) {
              if (obj.roles.includes(role)) {
                document.getElementById(role).classList.remove('disabled');
                if (sessionStorage.getItem('active') === role) {
                  document.getElementById(role).classList.add('active');
                }
              }
            }

            document.getElementById('userid').innerHTML = obj.user;
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
