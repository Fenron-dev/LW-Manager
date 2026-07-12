async function loadInfo() {
  try {
    const info = await window.go.main.App.GetAppInfo();
    document.querySelector('#platform').textContent = `${info.platform} · ${info.version}`;
    document.querySelector('#status').textContent = info.ready ? 'Vault ist bereit' : 'Vault benötigt Aufmerksamkeit';
    document.querySelector('#message').textContent = info.message;
    document.querySelector('#root').textContent = info.vaultRoot || 'nicht gefunden';
    document.querySelector('#indicator').textContent = info.ready ? '✓' : '!';
    document.querySelector('#indicator').classList.toggle('error', !info.ready);
  } catch (error) {
    document.querySelector('#status').textContent = 'Backend nicht erreichbar';
    document.querySelector('#message').textContent = String(error);
    document.querySelector('#indicator').textContent = '!';
    document.querySelector('#indicator').classList.add('error');
  }
}
loadInfo();
