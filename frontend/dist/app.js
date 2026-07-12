const $ = (selector) => document.querySelector(selector);
const scanButton = $('#scan-button');
let libraryPage = 1;
let libraryTotal = 0;
let libraryPageSize = 50;
let extensionsLoaded = false;

function formatBytes(bytes) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`;
}

function formatDate(value) {
  if (!value) return '–';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '–' : date.toLocaleDateString('de-DE');
}

function activateNavigation(active) {
  document.querySelectorAll('nav button').forEach((button) => button.classList.remove('active'));
  $(active).classList.add('active');
}

function showOverview() {
  $('#overview-view').classList.remove('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#page-title').textContent = 'Dein Vault auf einen Blick';
  activateNavigation('#nav-overview');
}

async function showLibrary() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.remove('hidden');
  $('#drives-view').classList.add('hidden');
  $('#page-title').textContent = 'Bibliothek';
  activateNavigation('#nav-library');
  await loadLibrary(1);
}

async function showDrives() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.remove('hidden');
  $('#page-title').textContent = 'Datenträger';
  activateNavigation('#nav-drives');
  await loadDrives();
}

async function loadInfo() {
  try {
    const info = await window.go.main.App.GetAppInfo();
    $('#platform').textContent = `${info.platform} · ${info.version}`;
    $('#status').textContent = info.ready ? 'Vault ist bereit' : 'Vault benötigt Aufmerksamkeit';
    $('#message').textContent = info.message;
    $('#root').textContent = info.vaultRoot || 'nicht gefunden';
    $('#indicator').textContent = info.ready ? '✓' : '!';
    $('#indicator').classList.toggle('error', !info.ready);
    $('#file-count').textContent = info.fileCount.toLocaleString('de-DE');
    $('#drive-count').textContent = info.driveCount.toLocaleString('de-DE');
    $('#file-caption').textContent = info.fileCount ? 'Metadaten im portablen Katalog' : 'Noch kein Scan durchgeführt';
    $('#drive-caption').textContent = info.driveCount ? 'Katalogisierte Quellen' : 'Keine Medien katalogisiert';
    scanButton.disabled = !info.ready;
  } catch (error) {
    $('#status').textContent = 'Backend nicht erreichbar';
    $('#message').textContent = String(error);
    $('#indicator').textContent = '!';
    $('#indicator').classList.add('error');
    scanButton.disabled = true;
  }
}

async function startScan() {
  showOverview();
  scanButton.disabled = true;
  $('#scan-title').textContent = 'Scan wird vorbereitet …';
  $('#scan-detail').textContent = 'Bitte den nativen Auswahldialog verwenden.';
  $('#progress').classList.add('active');
  try {
    const result = await window.go.main.App.SelectAndScan();
    if (!result.cancelled) {
      $('#scan-title').textContent = `${result.files.toLocaleString('de-DE')} Dateien katalogisiert`;
      $('#scan-detail').textContent = `${formatBytes(result.bytes)} erfasst · ${result.skipped} Einträge übersprungen`;
      extensionsLoaded = false;
      await loadInfo();
    }
  } catch (error) {
    $('#scan-title').textContent = 'Scan fehlgeschlagen';
    $('#scan-detail').textContent = String(error);
  } finally {
    $('#progress').classList.remove('active');
    scanButton.disabled = false;
  }
}

function renderFiles(files) {
  const container = $('#file-results');
  container.replaceChildren();
  for (const file of files) {
    const row = document.createElement('div');
    row.className = 'file-row';
    row.classList.add('file-clickable');
    const name = document.createElement('span');
    name.className = 'file-name';
    name.textContent = file.filename;
    name.title = file.filename;
    const drive = document.createElement('span');
    drive.className = 'file-drive';
    drive.textContent = file.drive;
    drive.title = file.drive;
    const path = document.createElement('span');
    path.className = 'file-path';
    path.textContent = file.path;
    path.title = path.textContent;
    const type = document.createElement('span');
    type.className = 'file-type';
    type.textContent = file.extension || 'Datei';
    const size = document.createElement('span');
    size.className = 'file-size';
    size.textContent = formatBytes(file.size);
    const date = document.createElement('span');
    date.className = 'file-date';
    date.textContent = formatDate(file.modified);
    row.append(name, drive, path, type, size, date);
    row.addEventListener('click', () => openFileDialog(file));
    container.append(row);
  }
}

async function loadLibrary(page = 1) {
  libraryPage = Math.max(1, page);
  $('#result-count').textContent = 'Suche läuft …';
  try {
    const result = await window.go.main.App.SearchFiles($('#search-input').value, $('#extension-filter').value, Number($('#drive-filter').value), libraryPage);
    libraryTotal = result.total;
    libraryPageSize = result.pageSize;
    renderFiles(result.files);
    if (!extensionsLoaded) {
      const filter = $('#extension-filter');
      const selected = filter.value;
      filter.replaceChildren(new Option('Alle Dateitypen', ''));
      result.extensions.forEach((extension) => filter.add(new Option(`.${extension}`, extension)));
      filter.value = selected;
      extensionsLoaded = true;
    }
    const pages = Math.max(1, Math.ceil(libraryTotal / libraryPageSize));
    $('#result-count').textContent = `${libraryTotal.toLocaleString('de-DE')} Treffer`;
    $('#page-label').textContent = `Seite ${libraryPage.toLocaleString('de-DE')} von ${pages.toLocaleString('de-DE')}`;
    $('#previous-page').disabled = libraryPage <= 1;
    $('#next-page').disabled = libraryPage >= pages;
    $('#library-empty').classList.toggle('hidden', result.files.length !== 0);
    $('.file-table').classList.toggle('hidden', result.files.length === 0);
  } catch (error) {
    $('#result-count').textContent = `Suche fehlgeschlagen: ${error}`;
  }
}

function driveName(drive) {
  return drive.displayName || drive.label;
}

async function loadDrives() {
  const drives = await window.go.main.App.GetDrives();
  const list = $('#drive-list');
  list.replaceChildren();
  $('#drives-empty').classList.toggle('hidden', drives.length !== 0);
  const filter = $('#drive-filter');
  const selectedDrive = filter.value;
  filter.replaceChildren(new Option('Alle Datenträger', '0'));
  for (const drive of drives) {
    filter.add(new Option(driveName(drive), String(drive.id)));
    const row = document.createElement('div');
    row.className = 'drive-row';
    const identity = document.createElement('div');
    identity.className = 'drive-identity';
    const heading = document.createElement('strong');
    heading.textContent = driveName(drive);
    const source = document.createElement('span');
    source.textContent = drive.inventoryNumber ? `Nr. ${drive.inventoryNumber} · ${drive.label}` : drive.label;
    identity.append(heading, source);
    const kind = document.createElement('span');
    kind.className = 'drive-cell';
    kind.textContent = [drive.manufacturer, drive.deviceType].filter(Boolean).join(' · ') || 'Nicht klassifiziert';
    const capacity = document.createElement('div');
    capacity.className = 'drive-capacity';
    const free = Math.max(0, drive.totalSize - drive.usedSize);
    const capacityText = document.createElement('span');
    capacityText.textContent = drive.totalSize ? `${formatBytes(drive.usedSize)} von ${formatBytes(drive.totalSize)} belegt` : 'Kapazität unbekannt';
    const bar = document.createElement('div');
    bar.className = 'capacity-bar';
    const fill = document.createElement('span');
    fill.style.width = drive.totalSize ? `${Math.min(100, drive.usedSize / drive.totalSize * 100)}%` : '0%';
    bar.append(fill);
    capacity.append(capacityText, bar);
    const files = document.createElement('span');
    files.className = 'drive-cell';
    files.textContent = `${drive.fileCount.toLocaleString('de-DE')} Dateien · ${formatBytes(free)} frei`;
    const edit = document.createElement('button');
    edit.className = 'secondary compact';
    edit.textContent = 'Bearbeiten';
    edit.addEventListener('click', () => openDriveDialog(drive));
    row.append(identity, kind, capacity, files, edit);
    list.append(row);
  }
  filter.value = [...filter.options].some((option) => option.value === selectedDrive) ? selectedDrive : '0';
}

function openDriveDialog(drive) {
  $('#edit-drive-id').value = drive.id;
  $('#drive-dialog-title').textContent = driveName(drive);
  $('#edit-display-name').value = drive.displayName || '';
  $('#edit-inventory-number').value = drive.inventoryNumber || '';
  $('#edit-manufacturer').value = drive.manufacturer || '';
  $('#edit-device-type').value = drive.deviceType || '';
  $('#drive-dialog-path').textContent = `Erkannt als ${drive.label} · ${drive.path}`;
  $('#drive-save-status').textContent = '';
  $('#drive-dialog').showModal();
}

async function saveDrive(event) {
  event.preventDefault();
  const button = $('#save-drive-button');
  button.disabled = true;
  try {
    await window.go.main.App.UpdateDrive(Number($('#edit-drive-id').value), $('#edit-display-name').value, $('#edit-inventory-number').value, $('#edit-manufacturer').value, $('#edit-device-type').value);
    $('#drive-save-status').textContent = 'Gespeichert ✓';
    await Promise.all([loadDrives(), loadInfo()]);
    setTimeout(() => $('#drive-dialog').close(), 350);
  } catch (error) {
    $('#drive-save-status').textContent = `Fehler: ${error}`;
  } finally { button.disabled = false; }
}

function openFileDialog(file) {
  $('#file-dialog-title').textContent = file.filename;
  $('#detail-drive').textContent = file.drive;
  $('#detail-path').textContent = file.path;
  $('#detail-type').textContent = file.mimeType || (file.extension ? `.${file.extension}` : 'Unbekannt');
  $('#detail-size').textContent = formatBytes(file.size);
  $('#detail-modified').textContent = formatDate(file.modified);
  $('#file-dialog').showModal();
}

window.runtime.EventsOn('scan:progress', (event) => {
  $('#scan-title').textContent = event.phase === 'save' ? 'Katalog wird gespeichert …' : `${event.files.toLocaleString('de-DE')} Dateien gefunden`;
  $('#scan-detail').textContent = event.path;
});
scanButton.addEventListener('click', startScan);
$('#nav-overview').addEventListener('click', showOverview);
$('#nav-library').addEventListener('click', showLibrary);
$('#nav-drives').addEventListener('click', showDrives);
$('#drive-scan-button').addEventListener('click', startScan);
$('#search-button').addEventListener('click', () => loadLibrary(1));
$('#search-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadLibrary(1); });
$('#extension-filter').addEventListener('change', () => loadLibrary(1));
$('#drive-filter').addEventListener('change', () => loadLibrary(1));
$('#previous-page').addEventListener('click', () => loadLibrary(libraryPage - 1));
$('#next-page').addEventListener('click', () => loadLibrary(libraryPage + 1));
$('#save-drive-button').addEventListener('click', saveDrive);
loadInfo().then(loadDrives);
