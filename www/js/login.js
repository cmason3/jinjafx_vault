(() => {
  let tid = 0;

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

      let xHR = new XMLHttpRequest();

      xHR.addEventListener('load', () => {
        if (xHR.status === 200) {
          let obj = JSON.parse(xHR.responseText);
          document.cookie = 'X-Vault-Token=' + obj.token;
          window.location.href = '/index.html';

        } else {
          setStatus(xHR.responseText);
        }
      });

      xHR.addEventListener('error', () => {
        setStatus('XMLHttpRequest().onError()');
      });

      request = {
        'user': document.getElementById('loginUser').value,
        'password': document.getElementById('loginPassword').value
      }
      xHR.open('POST', '/v1/login');
      xHR.send(JSON.stringify(request));
    });

    document.getElementById('loginUser').addEventListener('keyup', (e) => {
      if (e.key === 'Enter') {
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
