const $ = (selector) => document.querySelector(selector);
const scanButton = $('#scan-button');
let libraryPage = 1;
let libraryTotal = 0;
let libraryPageSize = 50;
let extensionsLoaded = false;
let activeSnapshot = 0;
let archivePage = 1;
let archivePages = 1;
let comparisonPage = 1;
let comparisonPages = 1;
let comparisonMode = 'list';

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

function withTimeout(promise, milliseconds, message) {
  let timeout;
  const timer = new Promise((_, reject) => { timeout = setTimeout(() => reject(new Error(message)), milliseconds); });
  return Promise.race([promise, timer]).finally(() => clearTimeout(timeout));
}

function activateNavigation(active) {
  document.querySelectorAll('nav button').forEach((button) => button.classList.remove('active'));
  $(active).classList.add('active');
}

function showOverview() {
  $('#overview-view').classList.remove('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.add('hidden');
  $('#page-title').textContent = 'Dein Vault auf einen Blick';
  activateNavigation('#nav-overview');
}

async function showLibrary() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.remove('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.add('hidden');
  $('#page-title').textContent = 'Bibliothek';
  activateNavigation('#nav-library');
  await loadLibrary(1);
}

async function showDrives() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.remove('hidden');
  $('#archive-view').classList.add('hidden');
  $('#page-title').textContent = 'Datenträger';
  activateNavigation('#nav-drives');
  await loadDrives();
}

async function showArchive() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.remove('hidden');
  $('#page-title').textContent = 'Archivvergleich';
  activateNavigation('#nav-archive');
  await loadDrives();
  await loadComparisonSnapshots();
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
  const compareDrive = $('#compare-drive');
  const selectedDrive = filter.value;
  const selectedCompareDrive = compareDrive.value;
  filter.replaceChildren(new Option('Alle Datenträger', '0'));
  compareDrive.replaceChildren(new Option('Datenträger auswählen', '0'));
  for (const drive of drives) {
    filter.add(new Option(driveName(drive), String(drive.id)));
    compareDrive.add(new Option(driveName(drive), String(drive.id)));
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
    const actions = document.createElement('div');
    actions.className = 'drive-row-actions';
    const history = document.createElement('button');
    history.className = 'secondary compact';
    history.textContent = 'Archivstände';
    history.addEventListener('click', (event) => { event.stopPropagation(); openArchiveDialog(drive); });
    const edit = document.createElement('button');
    edit.className = 'secondary compact';
    edit.textContent = 'Bearbeiten';
    edit.addEventListener('click', (event) => { event.stopPropagation(); openDriveDialog(drive); });
    actions.append(history, edit);
    row.append(identity, kind, capacity, files, actions);
    const treePanel = document.createElement('div');
    treePanel.className = 'drive-tree-panel hidden';
    row.addEventListener('click', async () => {
      const opening = treePanel.classList.contains('hidden');
      treePanel.classList.toggle('hidden', !opening);
      row.classList.toggle('expanded', opening);
      if (opening && !treePanel.dataset.loaded) {
        treePanel.dataset.loaded = 'true';
        treePanel.textContent = 'Verzeichnisstruktur wird geladen …';
        try {
          treePanel.replaceChildren(await createDirectoryLevel(drive.id, '', 0, driveName(drive)));
        } catch (error) {
          delete treePanel.dataset.loaded;
          treePanel.replaceChildren();
          const message = document.createElement('span');
          message.textContent = `Verzeichnis kann nicht geladen werden: ${error}`;
          const retry = document.createElement('button');
          retry.className = 'secondary compact';
          retry.textContent = 'Erneut versuchen';
          retry.addEventListener('click', (event) => { event.stopPropagation(); treePanel.classList.add('hidden'); row.click(); });
          treePanel.append(message, retry);
        }
      }
    });
    list.append(row, treePanel);
  }
  filter.value = [...filter.options].some((option) => option.value === selectedDrive) ? selectedDrive : '0';
  compareDrive.value = [...compareDrive.options].some((option) => option.value === selectedCompareDrive) ? selectedCompareDrive : '0';
}

async function loadComparisonSnapshots() {
  const driveID = Number($('#compare-drive').value);
  const select = $('#compare-snapshot');
  const selected = select.value;
  select.replaceChildren(new Option('Archivstand auswählen', '0'));
  if (!driveID) return;
  const snapshots = await window.go.main.App.GetDriveSnapshots(driveID);
  snapshots.forEach((snapshot) => select.add(new Option(`${formatDate(snapshot.capturedAt)} · ${snapshot.fileCount.toLocaleString('de-DE')} Dateien`, String(snapshot.id))));
  select.value = [...select.options].some((option) => option.value === selected) ? selected : (snapshots[0] ? String(snapshots[0].id) : '0');
}

async function loadComparison(page = 1) {
  const snapshotID = Number($('#compare-snapshot').value);
  if (!snapshotID) { $('#comparison-meta').textContent = 'Für diesen Datenträger ist noch kein Archivstand vorhanden.'; $('#comparison-results').replaceChildren(); return; }
  comparisonPage = Math.max(1,page);
  if (comparisonMode === 'tree') { await loadComparisonTree(); return; }
  $('.comparison-table').classList.remove('hidden');
  $('#comparison-tree').classList.add('hidden');
  $('#compare-pagination').classList.remove('hidden');
  $('#comparison-meta').textContent = 'Vergleich wird berechnet …';
  $('#compare-button').disabled = true;
  let result;
  try {
    result = await withTimeout(window.go.main.App.CompareSnapshot(snapshotID,$('#compare-status').value,$('#compare-query').value,comparisonPage),21000,'Zeitüberschreitung beim Archivvergleich');
  } catch (error) {
    $('#comparison-meta').textContent = `Vergleich fehlgeschlagen: ${error}`;
    $('#comparison-results').replaceChildren();
    return;
  } finally {
    $('#compare-button').disabled = false;
  }
  comparisonPages = Math.max(1,Math.ceil(result.total/result.pageSize));
  $('#comparison-meta').textContent = `${result.total.toLocaleString('de-DE')} Einträge · Seite ${comparisonPage} von ${comparisonPages}`;
  const container=$('#comparison-results');container.replaceChildren();
  const labels={added:'Neu',removed:'Entfernt',modified:'Geändert',unchanged:'Unverändert'};
  for(const entry of result.entries){
    const row=document.createElement('div');row.className=`comparison-row comparison-${entry.status}`;
    const current=document.createElement('div');current.className='compare-side';
    const currentName=document.createElement('strong');currentName.textContent=entry.currentName||'—';
    const currentMeta=document.createElement('span');currentMeta.textContent=entry.currentName?`${entry.path} · ${formatBytes(entry.currentSize)} · ${formatDate(entry.currentModified)}`:'Im aktuellen Stand nicht vorhanden';
    current.append(currentName,currentMeta);
    const status=document.createElement('span');status.className=`compare-status status-${entry.status}`;status.textContent=labels[entry.status]||entry.status;
    const archived=document.createElement('div');archived.className='compare-side';
    const archiveName=document.createElement('strong');archiveName.textContent=entry.archiveName||'—';
    const archiveMeta=document.createElement('span');archiveMeta.textContent=entry.archiveName?`${entry.path} · ${formatBytes(entry.archiveSize)} · ${formatDate(entry.archiveModified)}`:'Im Archivstand nicht vorhanden';
    archived.append(archiveName,archiveMeta);row.append(current,status,archived);container.append(row);
  }
  $('#compare-previous').disabled=comparisonPage<=1;$('#compare-next').disabled=comparisonPage>=comparisonPages;
}

async function loadComparisonTree() {
  const snapshotID=Number($('#compare-snapshot').value);if(!snapshotID){$('#comparison-meta').textContent='Für diesen Datenträger ist noch kein Archivstand vorhanden.';return}
  $('.comparison-table').classList.add('hidden');$('#compare-pagination').classList.add('hidden');
  const tree=$('#comparison-tree');tree.classList.remove('hidden');tree.textContent='Vergleichsbaum wird geladen …';
  $('#comparison-meta').textContent='Ordneränderungen werden zusammengefasst …';
  try{tree.replaceChildren(await createComparisonLevel(snapshotID,'',0));$('#comparison-meta').textContent='Ordner aufklappen, um Änderungen einzugrenzen.'}
  catch(error){tree.textContent=`Baumvergleich fehlgeschlagen: ${error}`;$('#comparison-meta').textContent='Vergleich konnte nicht geladen werden.'}
}

async function createComparisonLevel(snapshotID,directory,depth){
  const entries=await withTimeout(window.go.main.App.CompareSnapshotDirectory(snapshotID,directory,$('#compare-status').value),21000,'Zeitüberschreitung beim Baumvergleich');
  const level=document.createElement('div');level.className='compare-tree-level';
  for(const entry of entries){
    const row=document.createElement('div');row.className=`compare-tree-row tree-status-${entry.status}`;row.style.setProperty('--depth',depth);
    const toggle=document.createElement('span');toggle.className='tree-toggle';toggle.textContent=entry.isDir?'›':'·';
    const name=document.createElement('strong');name.textContent=entry.name;name.title=entry.path;
    const counts=document.createElement('span');counts.className='compare-tree-counts';
    const parts=[];if(entry.added)parts.push(`${entry.added} neu`);if(entry.removed)parts.push(`${entry.removed} entfernt`);if(entry.modified)parts.push(`${entry.modified} geändert`);if(entry.unchanged)parts.push(`${entry.unchanged} gleich`);counts.textContent=parts.join(' · ');
    row.append(toggle,name,counts);level.append(row);
    if(entry.isDir){const children=document.createElement('div');children.className='hidden';row.addEventListener('click',async()=>{const opening=children.classList.contains('hidden');children.classList.toggle('hidden',!opening);toggle.textContent=opening?'⌄':'›';if(opening&&!children.dataset.loaded){children.dataset.loaded='true';try{children.append(await createComparisonLevel(snapshotID,entry.path,depth+1))}catch(error){children.textContent=`Fehler: ${error}`}}});level.append(children)}
  }
  if(!entries.length){const empty=document.createElement('div');empty.className='tree-empty';empty.textContent='Keine Einträge für diesen Filter.';level.append(empty)}
  return level;
}

function setComparisonMode(mode){comparisonMode=mode;$('#compare-list-mode').classList.toggle('active',mode==='list');$('#compare-tree-mode').classList.toggle('active',mode==='tree');$('#compare-query').disabled=mode==='tree';loadComparison(1)}

async function openArchiveDialog(drive) {
  $('#archive-dialog-title').textContent = driveName(drive);
  $('#snapshot-list').classList.remove('hidden');
  $('#archive-browser').classList.add('hidden');
  $('#archive-dialog').showModal();
  await loadSnapshots(drive.id);
}

async function loadSnapshots(driveID) {
  const list = $('#snapshot-list');
  list.textContent = 'Archivstände werden geladen …';
  const snapshots = await window.go.main.App.GetDriveSnapshots(driveID);
  list.replaceChildren();
  if (!snapshots.length) {
    const empty = document.createElement('div');
    empty.className = 'tree-empty';
    empty.textContent = 'Noch keine älteren Scan-Stände. Beim nächsten Scan wird der aktuelle Stand hier archiviert.';
    list.append(empty);
    return;
  }
  for (const snapshot of snapshots) {
    const row = document.createElement('div');
    row.className = 'snapshot-row';
    const info = document.createElement('button');
    info.type = 'button';
    info.className = 'snapshot-open secondary';
    info.textContent = `${formatDate(snapshot.capturedAt)} · ${snapshot.fileCount.toLocaleString('de-DE')} Dateien · ${formatBytes(snapshot.totalBytes)}`;
    info.addEventListener('click', () => openSnapshot(snapshot.id));
    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'snapshot-delete';
    remove.textContent = 'Löschen';
    remove.addEventListener('click', async () => {
      if (!confirm(`Archivstand vom ${formatDate(snapshot.capturedAt)} wirklich unwiderruflich löschen?`)) return;
      await window.go.main.App.DeleteSnapshot(snapshot.id);
      await loadSnapshots(driveID);
    });
    row.append(info, remove);
    list.append(row);
  }
}

async function openSnapshot(snapshotID) {
  activeSnapshot = snapshotID;
  archivePage = 1;
  $('#snapshot-list').classList.add('hidden');
  $('#archive-browser').classList.remove('hidden');
  await loadArchiveFiles();
}

async function loadArchiveFiles(page = archivePage) {
  archivePage = Math.max(1, page);
  const result = await window.go.main.App.SearchSnapshot(activeSnapshot, $('#archive-search').value, archivePage);
  archivePages = Math.max(1, Math.ceil(result.total / result.pageSize));
  $('#archive-meta').textContent = `${result.total.toLocaleString('de-DE')} archivierte Dateien · Seite ${archivePage} von ${archivePages}`;
  const list = $('#archive-files');
  list.replaceChildren();
  for (const file of result.files) {
    const row = document.createElement('div');
    row.className = 'archive-file';
    const name = document.createElement('strong');
    name.textContent = file.filename;
    const path = document.createElement('span');
    path.textContent = file.path;
    const size = document.createElement('span');
    size.textContent = formatBytes(file.size);
    row.append(name, path, size);
    list.append(row);
  }
  $('#archive-previous').disabled = archivePage <= 1;
  $('#archive-next').disabled = archivePage >= archivePages;
}

async function createDirectoryLevel(driveID, directory, depth, driveLabel) {
  const entries = await withTimeout(window.go.main.App.BrowseDrive(driveID, directory), 13000, 'Zeitüberschreitung bei der Datenbankabfrage');
  const level = document.createElement('div');
  level.className = 'tree-level';
  for (const entry of entries) {
    const item = document.createElement('div');
    item.className = `tree-item ${entry.isDir ? 'tree-directory' : 'tree-file'}`;
    item.style.setProperty('--depth', depth);
    const toggle = document.createElement('span');
    toggle.className = 'tree-toggle';
    toggle.textContent = entry.isDir ? '›' : '·';
    const icon = document.createElement('span');
    icon.className = 'tree-icon';
    icon.textContent = entry.isDir ? '▰' : '▪';
    const name = document.createElement('span');
    name.className = 'tree-name';
    name.textContent = entry.name;
    name.title = entry.path;
    const meta = document.createElement('span');
    meta.className = 'tree-meta';
    meta.textContent = entry.isDir ? `${entry.fileCount.toLocaleString('de-DE')} Dateien · ${formatBytes(entry.size)}` : `${entry.extension ? `.${entry.extension} · ` : ''}${formatBytes(entry.size)}`;
    item.append(toggle, icon, name, meta);
    level.append(item);
    if (entry.isDir) {
      const children = document.createElement('div');
      children.className = 'tree-children hidden';
      item.addEventListener('click', async (event) => {
        event.stopPropagation();
        const opening = children.classList.contains('hidden');
        children.classList.toggle('hidden', !opening);
        toggle.textContent = opening ? '⌄' : '›';
        if (opening && !children.dataset.loaded) {
          children.dataset.loaded = 'true';
          children.append(await createDirectoryLevel(driveID, entry.path, depth + 1, driveLabel));
        }
      });
      level.append(children);
    } else {
      item.addEventListener('click', (event) => {
        event.stopPropagation();
        openFileDialog({id: entry.id, filename: entry.name, drive: driveLabel, path: entry.path, extension: entry.extension, mimeType: '', size: entry.size, modified: ''});
      });
    }
  }
  if (!entries.length) {
    const empty = document.createElement('div');
    empty.className = 'tree-empty';
    empty.textContent = 'Dieser Ordner ist leer.';
    level.append(empty);
  }
  return level;
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
  const previewWrap = $('#preview-wrap');
  const preview = $('#file-preview');
  const previewStatus = $('#preview-status');
  preview.removeAttribute('src');
  preview.classList.add('hidden');
  const previewable = file.id && (file.mimeType?.startsWith('image/') || ['jpg', 'jpeg', 'png', 'gif'].includes((file.extension || '').toLowerCase()));
  previewWrap.classList.toggle('hidden', !previewable);
  if (previewable) {
    previewStatus.classList.remove('hidden');
    previewStatus.textContent = 'Vorschau wird erzeugt …';
    window.go.main.App.GetImagePreview(file.id).then((dataURL) => {
      if (!$('#file-dialog').open) return;
      preview.src = dataURL;
      preview.classList.remove('hidden');
      previewStatus.classList.add('hidden');
    }).catch((error) => { previewStatus.textContent = `Keine Vorschau: ${error}`; });
  }
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
$('#nav-archive').addEventListener('click', showArchive);
$('#drive-scan-button').addEventListener('click', startScan);
$('#search-button').addEventListener('click', () => loadLibrary(1));
$('#search-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadLibrary(1); });
$('#extension-filter').addEventListener('change', () => loadLibrary(1));
$('#drive-filter').addEventListener('change', () => loadLibrary(1));
$('#previous-page').addEventListener('click', () => loadLibrary(libraryPage - 1));
$('#next-page').addEventListener('click', () => loadLibrary(libraryPage + 1));
$('#save-drive-button').addEventListener('click', saveDrive);
$('#archive-back').addEventListener('click', () => { $('#snapshot-list').classList.remove('hidden'); $('#archive-browser').classList.add('hidden'); });
$('#archive-search-button').addEventListener('click', () => loadArchiveFiles(1));
$('#archive-search').addEventListener('keydown', (event) => { if (event.key === 'Enter') { event.preventDefault(); loadArchiveFiles(1); } });
$('#archive-previous').addEventListener('click', () => loadArchiveFiles(archivePage - 1));
$('#archive-next').addEventListener('click', () => loadArchiveFiles(archivePage + 1));
$('#compare-drive').addEventListener('change', loadComparisonSnapshots);
$('#compare-button').addEventListener('click', () => loadComparison(1));
$('#compare-status').addEventListener('change', () => loadComparison(1));
$('#compare-snapshot').addEventListener('change', () => loadComparison(1));
$('#compare-query').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadComparison(1); });
$('#compare-previous').addEventListener('click', () => loadComparison(comparisonPage - 1));
$('#compare-next').addEventListener('click', () => loadComparison(comparisonPage + 1));
$('#compare-list-mode').addEventListener('click',()=>setComparisonMode('list'));
$('#compare-tree-mode').addEventListener('click',()=>setComparisonMode('tree'));
loadInfo().then(loadDrives);
