(() => {
  let active = null;

  function setStatus(message) {
    s = document.getElementById('status');
    s.innerHTML = message;
    new bootstrap.Modal(document.getElementById('error')).show();
  }

  async function buildPage() {
    let content = document.getElementById('content');
    content.innerHTML = '';

    try {
      if (active == 'admin') {
        let r = await fetch('/v1/users');
        if (r.status === 200) {
          let users = await r.json();
          content.innerHTML = '<pre>' + JSON.stringify(users, null, 2) + '</pre>';

        } else {
          setStatus(r.statusText);
        }
      } else {
        let r = await fetch('/v1/namespaces');
        if (r.status === 200) {
          let namespaces = await r.json();
          let data = {};

          for (let ns in namespaces) {
            let r = await fetch('/v1/data/' + ns);
            if (r.status === 200) {
              let namespace = await r.json();
              data[ns] = namespace;

            } else {
              setStatus(r.statusText);
              return;
            }
          }
          content.innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';

        } else {
          setStatus(r.statusText);
        }
      }
    } catch (error) {
      content.innerHTML = '';
      setStatus(error);
    }
  }

  window.addEventListener('load', async(e) => {
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
        fetch('/v1/logout', { method: 'POST' }).then((r) => {
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
      let r = await fetch('/v1/whoami');
      if (r.status === 200) {
        let obj = await r.json();

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

      } else {
        setStatus(r.statusText);
      }
    } catch (error) {
      setStatus(error);
    }
  });
})();
