(() => {
  let tid = 0;

  function setStatus(message) {
    clearTimeout(tid);
    s = document.getElementById('status');
    s.innerHTML = message;
    s.style.transition = "none";
    s.style.opacity = 1;
    tid = setTimeout(function() {
      s.style.transition = "all 1.0s";
      s.style.opacity = 0;
    }, 4000);
  }

  window.addEventListener('load', (e) => {
    document.getElementById('login').addEventListener('shown.bs.modal', (e) => {
      document.getElementById('loginUser').focus();
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
          setStatus('Error: ' + xHR.responseText);
          document.getElementById('loginPassword').focus();
        }
      });

      xHR.addEventListener('error', () => {
        setStatus('Error: XMLHttpRequest().onError()');
        document.getElementById('loginPassword').focus();
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

    new bootstrap.Modal(document.getElementById('login'), { keyboard: false }).show();
  });
})();
