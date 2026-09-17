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
    document.getElementById('chpass').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('loginPassword').focus();
    });

    document.getElementById('error').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('error').focus();
    });

    document.getElementById('error').addEventListener('hidden.bs.modal', (e) => {
      document.getElementById('loginPassword').focus();
    });

    document.getElementById('submit').addEventListener('click', (e) => {
      if (document.getElementById('loginPassword').value.trim().length === 0) {
        document.getElementById('loginPassword').focus();
        return;

      } else if (document.getElementById('loginNewPassword').value.trim().length === 0) {
        document.getElementById('loginNewPassword').focus();
        return;

      } else if (document.getElementById('loginVerifyPassword').value.trim().length === 0) {
        document.getElementById('loginVerifyPassword').focus();
        return;

      } else if (document.getElementById('loginNewPassword').value != document.getElementById('loginVerifyPassword').value) {
        setStatus('<b>HTTP 400</b> Password Verification Failed');
        document.getElementById('loginVerifyPassword').focus();
        return;
      }

      try {
        let request = {
          'old_password': document.getElementById('loginPassword').value,
          'password': document.getElementById('loginNewPassword').value
        };

        fetch('v1/chpass', { method: 'POST', body: JSON.stringify(request), signal: AbortSignal.timeout(timeout) }).then((r) => {
          if (r.status === 204) {
            window.location.href = 'login.html';

          } else {
            r.text().then((msg) => {
              setStatus('<b>HTTP ' + r.status + '</b> ' + msg);
              document.getElementById('loginPassword').focus();
            });
          }
        });

      } catch (error) {
        setStatus(error);
      }
    });

    document.getElementById('loginPassword').addEventListener('keyup', (e) => {
      if ((e.key === 'Enter') && (document.getElementById('loginPassword').value.trim().length !== 0)) {
        document.getElementById('loginNewPassword').focus();
      }
    });

    document.getElementById('loginNewPassword').addEventListener('keyup', (e) => {
      if ((e.key === 'Enter') && (document.getElementById('loginNewPassword').value.trim().length !== 0)) {
        document.getElementById('loginVerifyPassword').focus();
      }
    });

    document.getElementById('loginVerifyPassword').addEventListener('keyup', (e) => {
      if (e.key === 'Enter') {
        document.getElementById('submit').click();
      }
    });

    new bootstrap.Modal(document.getElementById('chpass'), { keyboard: false }).show();
  });
})();
