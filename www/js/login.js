(() => {
  let tid = 0;
  let timeout = 5000;

  function setStatus(message) {
    clearTimeout(tid);
    s = document.getElementById('status');
    s.innerHTML = message;
    let m = new bootstrap.Modal(document.getElementById('error'));
    m.show();
    tid = setTimeout(function() { m.hide() }, 4000);
  }

  window.addEventListener('load', (e) => {
    document.getElementById('login').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('loginUser').focus();
    });

    document.getElementById('error').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('error').focus();
    });

    document.getElementById('error').addEventListener('hidden.bs.modal', (e) => {
      document.getElementById('loginPassword').focus();
    });

    document.getElementById('submit').addEventListener('click', (e) => {
      if (document.getElementById('loginUser').value.trim().length === 0) {
        document.getElementById('loginUser').focus();
        return;

      } else if (document.getElementById('loginPassword').value.trim().length === 0) {
        document.getElementById('loginPassword').focus();
        return;
      }

      try {
        let request = {
          'user': document.getElementById('loginUser').value,
          'password': document.getElementById('loginPassword').value
        };
        fetch('/v1/login', { method: 'POST', body: JSON.stringify(request), signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 200) {
            r.json().then((obj) => {
              document.cookie = 'X-Vault-Token=' + obj.token;
              window.location.href = 'index.html';
            });

          } else {
            setStatus(r.statusText);
          }
        });

      } catch (error) {
        setStatus(error);
      }
    });

    document.getElementById('loginUser').addEventListener('keyup', (e) => {
      if ((e.key === 'Enter') && (document.getElementById('loginUser').value.trim().length !== 0)) {
        document.getElementById('loginPassword').focus();
      }
    });

    document.getElementById('loginPassword').addEventListener('keyup', (e) => {
      if (e.key === 'Enter') {
        document.getElementById('submit').click();
      }
    });

    document.cookie = 'X-Vault-Token=; expires=Thu, 01 Jan 1970 00:00:00 UTC;';
    new bootstrap.Modal(document.getElementById('login'), { keyboard: false }).show();
  });
})();
