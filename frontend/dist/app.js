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

function createDriveField(label, value, name, options) {
  const wrapper = document.createElement('div');
  wrapper.className = 'drive-field';
  const caption = document.createElement('label');
  caption.textContent = label;
  let input;
  if (options) {
    input = document.createElement('select');
    input.add(new Option('Nicht festgelegt', ''));
    options.forEach((option) => input.add(new Option(option, option)));
  } else {
    input = document.createElement('input');
  }
  input.name = name;
  input.value = value || '';
  wrapper.append(caption, input);
  return wrapper;
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
    const card = document.createElement('article');
    card.className = 'drive-card';
    card.dataset.id = drive.id;
    const head = document.createElement('div');
    head.className = 'drive-card-head';
    const title = document.createElement('div');
    const heading = document.createElement('h3');
    heading.textContent = driveName(drive);
    const source = document.createElement('p');
    source.textContent = `Erkannt als ${drive.label} · ${drive.path}`;
    source.title = drive.path;
    title.append(heading, source);
    const badge = document.createElement('span');
    badge.className = 'drive-badge';
    badge.textContent = `${drive.fileCount.toLocaleString('de-DE')} Dateien`;
    head.append(title, badge);
    const fields = document.createElement('div');
    fields.className = 'drive-fields';
    fields.append(
      createDriveField('Eigener Name', drive.displayName, 'displayName'),
      createDriveField('Inventarnummer', drive.inventoryNumber, 'inventoryNumber'),
      createDriveField('Hersteller', drive.manufacturer, 'manufacturer'),
      createDriveField('Bauart', drive.deviceType, 'deviceType', ['2,5″ HDD', '2,5″ SSD', 'M.2 SATA SSD', 'M.2 NVMe SSD', 'USB-A Stick', 'USB-C Stick', 'SD-Karte', 'Sonstiges']),
    );
    const stats = document.createElement('div');
    stats.className = 'drive-stats';
    const free = Math.max(0, drive.totalSize - drive.usedSize);
    [['Kapazität', drive.totalSize], ['Belegt', drive.usedSize], ['Frei', free]].forEach(([label, value]) => {
      const stat = document.createElement('div');
      stat.className = 'drive-stat';
      const strong = document.createElement('strong');
      strong.textContent = value ? formatBytes(value) : 'Unbekannt';
      stat.append(strong, document.createTextNode(label));
      stats.append(stat);
    });
    const actions = document.createElement('div');
    actions.className = 'drive-card-actions';
    const save = document.createElement('button');
    save.textContent = 'Angaben speichern';
    save.addEventListener('click', async () => {
      save.disabled = true;
      try {
        const values = Object.fromEntries([...fields.querySelectorAll('input,select')].map((input) => [input.name, input.value]));
        await window.go.main.App.UpdateDrive(drive.id, values.displayName, values.inventoryNumber, values.manufacturer, values.deviceType);
        save.textContent = 'Gespeichert ✓';
        setTimeout(() => { save.textContent = 'Angaben speichern'; }, 1600);
        await loadInfo();
      } catch (error) {
        save.textContent = `Fehler: ${error}`;
      } finally { save.disabled = false; }
    });
    actions.append(save);
    card.append(head, fields, stats, actions);
    list.append(card);
  }
  filter.value = [...filter.options].some((option) => option.value === selectedDrive) ? selectedDrive : '0';
}

window.runtime.EventsOn('scan:progress', (event) => {
  $('#scan-title').textContent = event.phase === 'save' ? 'Katalog wird gespeichert …' : `${event.files.toLocaleString('de-DE')} Dateien gefunden`;
  $('#scan-detail').textContent = event.path;
});
scanButton.addEventListener('click', startScan);
$('#nav-scan').addEventListener('click', startScan);
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
loadInfo().then(loadDrives);
