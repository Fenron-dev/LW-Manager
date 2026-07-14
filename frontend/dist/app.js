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
let duplicateGroups = [];
let duplicatePage = 1;
const duplicatePageSize = 25;

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
  document.querySelectorAll('aside button').forEach((button) => button.classList.remove('active'));
  $(active).classList.add('active');
}

function showOverview() {
  $('#overview-view').classList.remove('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.add('hidden');
  $('#settings-view').classList.add('hidden');
  $('#page-title').textContent = 'Dein Vault auf einen Blick';
  activateNavigation('#nav-overview');
}

async function showLibrary() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.remove('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.add('hidden');
  $('#settings-view').classList.add('hidden');
  $('#page-title').textContent = 'Bibliothek';
  activateNavigation('#nav-library');
  await loadLibrary(1);
}

async function showDrives() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.remove('hidden');
  $('#archive-view').classList.add('hidden');
  $('#settings-view').classList.add('hidden');
  $('#page-title').textContent = 'Datenträger';
  activateNavigation('#nav-drives');
  await loadDrives();
}

async function showArchive() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.remove('hidden');
  $('#settings-view').classList.add('hidden');
  $('#page-title').textContent = 'Archivvergleich';
  activateNavigation('#nav-archive');
  await loadDrives();
  await loadComparisonSnapshots();
}

async function showSettings() {
  $('#overview-view').classList.add('hidden');
  $('#library-view').classList.add('hidden');
  $('#drives-view').classList.add('hidden');
  $('#archive-view').classList.add('hidden');
  $('#settings-view').classList.remove('hidden');
  $('#page-title').textContent = 'Einstellungen';
  activateNavigation('#nav-settings');
  $('#settings-status').textContent = '';
  try {
    const settings = await window.go.main.App.GetSettings();
    $('#setting-volume-detection').checked = settings.volumeDetectionEnabled;
    $('#setting-backup-enabled').checked = settings.backupEnabled;
    $('#setting-backup-thumbnails').checked = settings.backupIncludeThumbnails;
    $('#setting-backup-file-limit').value = settings.backupFileMB;
    $('#setting-backup-file-unlimited').checked = settings.backupFileUnlimited;
    $('#setting-backup-limit').value = settings.backupMaxMB;
    $('#setting-backup-unlimited').checked = settings.backupUnlimited;
    $('#setting-archive-enabled').checked = settings.archiveEnabled;
    $('#setting-max-snapshots').value = settings.maxSnapshots;
    $('#setting-image-analysis-enabled').checked = settings.imageAnalysisEnabled;
    $('#setting-image-jpeg').checked = settings.imageJPEGEnabled;
    $('#setting-image-png').checked = settings.imagePNGEnabled;
    $('#setting-image-gif').checked = settings.imageGIFEnabled;
    $('#setting-image-heic').checked = settings.imageHEICEnabled;
    $('#setting-image-header-limit').value = settings.imageHeaderMB;
    $('#setting-image-header-unlimited').checked = settings.imageHeaderUnlimited;
    $('#setting-image-scan-limit').value = settings.imageScanBudgetMB;
    $('#setting-image-scan-unlimited').checked = settings.imageScanBudgetUnlimited;
    $('#setting-exif-enabled').checked = settings.exifEnabled;
    $('#setting-exif-file-limit').value = settings.exifFileMB;
    $('#setting-exif-file-unlimited').checked = settings.exifFileUnlimited;
    $('#setting-exif-total-limit').value = settings.exifTotalMB;
    $('#setting-exif-total-unlimited').checked = settings.exifTotalUnlimited;
    $('#setting-text-enabled').checked = settings.textIndexEnabled;
    $('#setting-text-documents').checked = settings.textDocumentsEnabled;
    $('#setting-text-data').checked = settings.textDataEnabled;
    $('#setting-text-source').checked = settings.textSourceEnabled;
    $('#setting-text-file-limit').value = settings.textFileMB;
    $('#setting-text-file-unlimited').checked = settings.textFileUnlimited;
    $('#setting-text-total-limit').value = settings.textTotalMB;
    $('#setting-text-total-unlimited').checked = settings.textTotalUnlimited;
    $('#setting-image-preview-enabled').checked = settings.imagePreviewEnabled;
    $('#setting-heic-preview').checked = settings.heicPreviewEnabled;
    $('#setting-image-limit').value = settings.imagePreviewMB;
    $('#setting-image-preview-unlimited').checked = settings.imagePreviewUnlimited;
    $('#setting-thumbnail-cache-limit').value = settings.thumbnailCacheMB;
    $('#setting-thumbnail-cache-unlimited').checked = settings.thumbnailCacheUnlimited;
    $('#setting-pdf-limit').value = settings.pdfPreviewMB;
    $('#setting-video-limit').value = settings.videoPreviewMB;
    syncSettingsControls();
  } catch (error) { $('#settings-status').textContent = `Fehler: ${error}`; }
}

async function saveSettings() {
  const button = $('#save-settings-button');
  button.disabled = true;
  let saved = false;
  try {
    await window.go.main.App.SaveSettings({
      version: 7,
      volumeDetectionEnabled: $('#setting-volume-detection').checked,
      backupEnabled: $('#setting-backup-enabled').checked,
      backupIncludeThumbnails: $('#setting-backup-thumbnails').checked,
      backupFileMB: Number($('#setting-backup-file-limit').value),
      backupFileUnlimited: $('#setting-backup-file-unlimited').checked,
      backupMaxMB: Number($('#setting-backup-limit').value),
      backupUnlimited: $('#setting-backup-unlimited').checked,
      archiveEnabled: $('#setting-archive-enabled').checked,
      maxSnapshots: Number($('#setting-max-snapshots').value),
      imageAnalysisEnabled: $('#setting-image-analysis-enabled').checked,
      imageJPEGEnabled: $('#setting-image-jpeg').checked,
      imagePNGEnabled: $('#setting-image-png').checked,
      imageGIFEnabled: $('#setting-image-gif').checked,
      imageHEICEnabled: $('#setting-image-heic').checked,
      imageHeaderMB: Number($('#setting-image-header-limit').value),
      imageHeaderUnlimited: $('#setting-image-header-unlimited').checked,
      imageScanBudgetMB: Number($('#setting-image-scan-limit').value),
      imageScanBudgetUnlimited: $('#setting-image-scan-unlimited').checked,
      exifEnabled: $('#setting-exif-enabled').checked,
      exifFileMB: Number($('#setting-exif-file-limit').value),
      exifFileUnlimited: $('#setting-exif-file-unlimited').checked,
      exifTotalMB: Number($('#setting-exif-total-limit').value),
      exifTotalUnlimited: $('#setting-exif-total-unlimited').checked,
      textIndexEnabled: $('#setting-text-enabled').checked,
      textDocumentsEnabled: $('#setting-text-documents').checked,
      textDataEnabled: $('#setting-text-data').checked,
      textSourceEnabled: $('#setting-text-source').checked,
      textFileMB: Number($('#setting-text-file-limit').value),
      textFileUnlimited: $('#setting-text-file-unlimited').checked,
      textTotalMB: Number($('#setting-text-total-limit').value),
      textTotalUnlimited: $('#setting-text-total-unlimited').checked,
      imagePreviewEnabled: $('#setting-image-preview-enabled').checked,
      heicPreviewEnabled: $('#setting-heic-preview').checked,
      imagePreviewMB: Number($('#setting-image-limit').value),
      imagePreviewUnlimited: $('#setting-image-preview-unlimited').checked,
      thumbnailCacheMB: Number($('#setting-thumbnail-cache-limit').value),
      thumbnailCacheUnlimited: $('#setting-thumbnail-cache-unlimited').checked,
      pdfPreviewMB: Number($('#setting-pdf-limit').value),
      videoPreviewMB: Number($('#setting-video-limit').value)
    });
    $('#settings-status').textContent = 'Einstellungen gespeichert ✓';
    saved = true;
  } catch (error) { $('#settings-status').textContent = `Fehler: ${error}`; }
  finally { button.disabled = false; }
  return saved;
}

async function createBackup() {
  const button = $('#create-backup-button');
  button.disabled = true;
  if (!await saveSettings()) {
    syncSettingsControls();
    return;
  }
  $('#settings-status').textContent = 'Backup wird vorbereitet …';
  try {
    const result = await window.go.main.App.CreateBackup();
    if (result.cancelled) {
      $('#settings-status').textContent = 'Backup abgebrochen.';
    } else {
      $('#settings-status').textContent = `${result.message}: ${formatBytes(result.bytes)} · ${result.files.toLocaleString('de-DE')} Dateien`;
    }
  } catch (error) {
    $('#settings-status').textContent = `Backup fehlgeschlagen: ${error}`;
  } finally {
    syncSettingsControls();
  }
}

function syncSettingsControls() {
  const backupEnabled = $('#setting-backup-enabled').checked;
  $('#setting-backup-thumbnails').disabled = !backupEnabled;
  $('#setting-backup-file-unlimited').disabled = !backupEnabled;
  $('#setting-backup-file-limit').disabled = !backupEnabled || $('#setting-backup-file-unlimited').checked;
  $('#setting-backup-unlimited').disabled = !backupEnabled;
  $('#setting-backup-limit').disabled = !backupEnabled || $('#setting-backup-unlimited').checked;
  $('#create-backup-button').disabled = !backupEnabled;
  const analysisEnabled = $('#setting-image-analysis-enabled').checked;
  ['#setting-image-jpeg', '#setting-image-png', '#setting-image-gif', '#setting-image-heic', '#setting-image-header-unlimited', '#setting-image-scan-unlimited'].forEach((selector) => { $(selector).disabled = !analysisEnabled; });
  $('#setting-image-header-limit').disabled = !analysisEnabled || $('#setting-image-header-unlimited').checked;
  $('#setting-image-scan-limit').disabled = !analysisEnabled || $('#setting-image-scan-unlimited').checked;
  const exifEnabled = $('#setting-exif-enabled').checked;
  ['#setting-exif-file-unlimited', '#setting-exif-total-unlimited'].forEach((selector) => { $(selector).disabled = !exifEnabled; });
  $('#setting-exif-file-limit').disabled = !exifEnabled || $('#setting-exif-file-unlimited').checked;
  $('#setting-exif-total-limit').disabled = !exifEnabled || $('#setting-exif-total-unlimited').checked;
  const textEnabled = $('#setting-text-enabled').checked;
  ['#setting-text-documents', '#setting-text-data', '#setting-text-source', '#setting-text-file-unlimited', '#setting-text-total-unlimited'].forEach((selector) => { $(selector).disabled = !textEnabled; });
  $('#setting-text-file-limit').disabled = !textEnabled || $('#setting-text-file-unlimited').checked;
  $('#setting-text-total-limit').disabled = !textEnabled || $('#setting-text-total-unlimited').checked;
  const previewEnabled = $('#setting-image-preview-enabled').checked;
  ['#setting-heic-preview', '#setting-image-preview-unlimited', '#setting-thumbnail-cache-unlimited'].forEach((selector) => { $(selector).disabled = !previewEnabled; });
  $('#setting-image-limit').disabled = !previewEnabled || $('#setting-image-preview-unlimited').checked;
  $('#setting-thumbnail-cache-limit').disabled = !previewEnabled || $('#setting-thumbnail-cache-unlimited').checked;
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

async function runScan(scanAction, preparingMessage) {
  showOverview();
  scanButton.disabled = true;
  $('#scan-title').textContent = 'Scan wird vorbereitet …';
  $('#scan-detail').textContent = preparingMessage;
  $('#progress').classList.add('active');
  try {
    const result = await scanAction();
    if (!result.cancelled) {
      $('#scan-title').textContent = `${result.files.toLocaleString('de-DE')} Dateien katalogisiert`;
      $('#scan-detail').textContent = `${formatBytes(result.bytes)} erfasst · ${result.skipped} Einträge übersprungen`;
      extensionsLoaded = false;
      await Promise.all([loadInfo(), loadDrives()]);
    }
  } catch (error) {
    $('#scan-title').textContent = 'Scan fehlgeschlagen';
    $('#scan-detail').textContent = String(error);
  } finally {
    $('#progress').classList.remove('active');
    scanButton.disabled = false;
  }
}

async function startScan() {
  return runScan(() => window.go.main.App.SelectAndScan(), 'Bitte den nativen Auswahldialog verwenden.');
}

async function startVolumeScan(volume) {
  return runScan(() => window.go.main.App.ScanVolume(volume.path), `${volume.label || volume.path} wird vorbereitet.`);
}

function renderFiles(files) {
  const container = $('#file-results');
  container.replaceChildren();
  for (const file of files) {
    const row = document.createElement('div');
    row.className = 'file-row';
    row.classList.add('file-clickable');
    const name = document.createElement('div');
    name.className = 'file-name';
    name.textContent = file.filename;
    name.title = file.filename;
    if (file.matchSnippet) {
      const snippet = document.createElement('small');
      snippet.className = 'content-match-snippet';
      snippet.textContent = file.matchSnippet.replace(/\s+/g, ' ').trim();
      name.append(snippet);
    }
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
    const result = await window.go.main.App.SearchFiles($('#search-input').value, $('#extension-filter').value, Number($('#drive-filter').value), $('#content-search').checked, libraryPage);
    libraryTotal = result.total;
    libraryPageSize = result.pageSize;
    renderFiles(result.files);
    if (!extensionsLoaded) {
      const filter = $('#extension-filter');
      const selected = filter.value;
      filter.replaceChildren(new Option('Alle Dateitypen', ''));
      result.extensions.sort((left, right) => left.localeCompare(right, 'de', {sensitivity: 'base'})).forEach((extension) => filter.add(new Option(`.${extension}`, extension)));
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

function parseTags(value) {
  return [...new Set(String(value || '').split(',').map((tag) => tag.trim()).filter(Boolean))];
}

function appendTagBadges(container, tags = []) {
  if (!tags.length) return;
  const badges = document.createElement('span');
  badges.className = 'tag-badges';
  tags.forEach((tag) => { const badge = document.createElement('em'); badge.textContent = tag; badges.append(badge); });
  container.append(badges);
}

async function loadDrives() {
  const drives = await window.go.main.App.GetDrives();
  await loadConnectedVolumes(drives);
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
    source.textContent = [drive.inventoryNumber ? `Nr. ${drive.inventoryNumber}` : '', drive.label, drive.storageLocation ? `Lager: ${drive.storageLocation}` : ''].filter(Boolean).join(' · ');
    identity.append(heading, source);
    appendTagBadges(identity, drive.tags);
    const kind = document.createElement('span');
    kind.className = 'drive-cell';
    kind.textContent = [drive.manufacturer || drive.model, drive.deviceType, drive.fsType].filter(Boolean).join(' · ') || 'Nicht klassifiziert';
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

function sameVolume(drive, volume) {
  const driveUUID = String(drive.uuid || '').toLowerCase().replace(/^volume:/, '');
  const volumeUUID = String(volume.uuid || '').toLowerCase().replace(/^volume:/, '');
  if (driveUUID && volumeUUID && driveUUID === volumeUUID) return true;
  return String(drive.path || '').replace(/[\\/]+$/, '').toLowerCase() === String(volume.path || '').replace(/[\\/]+$/, '').toLowerCase();
}

async function loadConnectedVolumes(drives = []) {
  const list = $('#connected-volume-list');
  const empty = $('#connected-volumes-empty');
  list.replaceChildren();
  empty.classList.remove('hidden');
  empty.textContent = 'Angeschlossene Datenträger werden gesucht …';
  try {
    const result = await window.go.main.App.GetConnectedVolumes();
    if (!result.enabled) {
      empty.textContent = 'Die automatische Datenträgererkennung ist in den Einstellungen deaktiviert.';
      return;
    }
    const volumes = result.volumes || [];
    empty.classList.toggle('hidden', volumes.length !== 0);
    empty.textContent = 'Keine externen Datenträger erkannt.';
    for (const volume of volumes) {
      const known = drives.find((drive) => sameVolume(drive, volume));
      const row = document.createElement('div');
      row.className = 'connected-volume-row';
      const identity = document.createElement('div');
      identity.className = 'drive-identity';
      const name = document.createElement('strong');
      name.textContent = volume.label || volume.path;
      const path = document.createElement('span');
      path.textContent = [volume.path, volume.fsType].filter(Boolean).join(' · ');
      identity.append(name, path);
      const capacity = document.createElement('div');
      capacity.className = 'drive-capacity';
      const capacityText = document.createElement('span');
      capacityText.textContent = volume.totalSize ? `${formatBytes(volume.usedSize)} von ${formatBytes(volume.totalSize)} belegt` : 'Kapazität unbekannt';
      const bar = document.createElement('div');
      bar.className = 'capacity-bar';
      const fill = document.createElement('span');
      fill.style.width = volume.totalSize ? `${Math.min(100, volume.usedSize / volume.totalSize * 100)}%` : '0%';
      bar.append(fill);
      capacity.append(capacityText, bar);
      const state = document.createElement('span');
      state.className = `volume-state ${known ? 'known' : ''}`;
      state.textContent = known ? `Katalogisiert als ${driveName(known)}` : 'Noch nicht katalogisiert';
      const scan = document.createElement('button');
      scan.className = 'compact';
      scan.textContent = known ? 'Neu scannen' : 'Scannen';
      scan.addEventListener('click', () => startVolumeScan(volume));
      row.append(identity, capacity, state, scan);
      list.append(row);
    }
  } catch (error) {
    empty.classList.remove('hidden');
    empty.textContent = `Datenträgererkennung fehlgeschlagen: ${error}`;
  }
}

async function loadComparisonSnapshots() {
  const driveID = Number($('#compare-drive').value);
  const select = $('#compare-snapshot');
  const selected = select.value;
  select.replaceChildren(new Option('Archivstand auswählen', '0'));
  if (!driveID) return;
  const snapshots = await window.go.main.App.GetDriveSnapshots(driveID);
  snapshots.forEach((snapshot) => select.add(new Option(`${snapshot.protected ? '🔒 ' : ''}${formatDate(snapshot.capturedAt)} · ${snapshot.fileCount.toLocaleString('de-DE')} Dateien${snapshot.tags?.length ? ` · ${snapshot.tags.join(', ')}` : ''}`, String(snapshot.id))));
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
    if (snapshot.protected) row.classList.add('protected');
    const head = document.createElement('div');
    head.className = 'snapshot-head';
    const info = document.createElement('button');
    info.type = 'button';
    info.className = 'snapshot-open secondary';
    info.textContent = `${snapshot.protected ? '🔒 ' : ''}${formatDate(snapshot.capturedAt)} · ${snapshot.fileCount.toLocaleString('de-DE')} Dateien · ${formatBytes(snapshot.totalBytes)}`;
    info.addEventListener('click', () => openSnapshot(snapshot.id));
    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'snapshot-delete';
    remove.textContent = 'Löschen';
    remove.disabled = snapshot.protected;
    remove.title = snapshot.protected ? 'Geschützte Archivstände können nicht gelöscht werden' : '';
    remove.addEventListener('click', async () => {
      if (!confirm(`Archivstand vom ${formatDate(snapshot.capturedAt)} wirklich unwiderruflich löschen?`)) return;
      await window.go.main.App.DeleteSnapshot(snapshot.id);
      await loadSnapshots(driveID);
    });
    const protection = document.createElement('label');
    protection.className = 'setting-toggle snapshot-protection';
    const toggle = document.createElement('input');
    toggle.type = 'checkbox'; toggle.checked = snapshot.protected;
    protection.append(toggle, document.createTextNode('Nicht löschbar'));
    const fields = document.createElement('div'); fields.className = 'snapshot-fields';
    const noteLabel = document.createElement('label'); noteLabel.textContent = 'Bemerkung';
    const note = document.createElement('textarea'); note.rows = 2; note.value = snapshot.note || ''; note.placeholder = 'Bemerkung zu diesem Scan-Stand'; noteLabel.append(note);
    const tagLabel = document.createElement('label'); tagLabel.textContent = 'Tags';
    const tags = document.createElement('input'); tags.value = (snapshot.tags || []).join(', '); tags.placeholder = 'z. B. Referenz, Übergabe'; tagLabel.append(tags);
    const save = document.createElement('button'); save.type = 'button'; save.className = 'secondary compact'; save.textContent = 'Angaben speichern';
    const status = document.createElement('span'); status.className = 'snapshot-save-status';
    const persist = async () => {
      save.disabled = true; toggle.disabled = true; status.textContent = 'Wird gespeichert …';
      try {
        await window.go.main.App.UpdateSnapshot(snapshot.id, toggle.checked, note.value, parseTags(tags.value));
        status.textContent = 'Gespeichert ✓'; remove.disabled = toggle.checked; row.classList.toggle('protected', toggle.checked);
        info.textContent = `${toggle.checked ? '🔒 ' : ''}${formatDate(snapshot.capturedAt)} · ${snapshot.fileCount.toLocaleString('de-DE')} Dateien · ${formatBytes(snapshot.totalBytes)}`;
        snapshot.protected = toggle.checked; snapshot.note = note.value; snapshot.tags = parseTags(tags.value);
        loadComparisonSnapshots().catch(() => {});
      } catch (error) { status.textContent = `Fehler: ${error}`; toggle.checked = snapshot.protected; }
      finally { save.disabled = false; toggle.disabled = false; }
    };
    toggle.addEventListener('change', persist); save.addEventListener('click', persist);
    head.append(info, protection, remove); fields.append(noteLabel, tagLabel, save, status); row.append(head, fields);
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

async function openDriveDialog(drive) {
  $('#edit-drive-id').value = drive.id;
  $('#drive-dialog-title').textContent = driveName(drive);
  $('#edit-display-name').value = drive.displayName || '';
  $('#edit-inventory-number').value = drive.inventoryNumber || '';
  $('#edit-manufacturer').value = drive.manufacturer || '';
  $('#edit-device-type').value = drive.deviceType || '';
  $('#edit-drive-note').value = drive.note || '';
  $('#edit-drive-tags').value = (drive.tags || []).join(', ');
  await loadStorageLocations(drive.storageLocation || '');
  $('#drive-detail-uuid').textContent = drive.uuid || 'Nicht verfügbar';
  $('#drive-detail-serial').textContent = drive.serial || 'Vom Datenträger nicht gemeldet';
  $('#drive-detail-vendor').textContent = drive.vendor || 'Nicht verfügbar';
  $('#drive-detail-model').textContent = drive.model || 'Nicht verfügbar';
  $('#drive-detail-fstype').textContent = drive.fsType || 'Nicht verfügbar';
  $('#drive-detail-connection').textContent = drive.detectedType || 'Nicht verfügbar';
  $('#drive-detail-path').textContent = `${drive.label} · ${drive.path}`;
  $('#drive-save-status').textContent = '';
  $('#drive-dialog').showModal();
}

async function saveDrive(event) {
  event.preventDefault();
  const button = $('#save-drive-button');
  button.disabled = true;
  try {
    await window.go.main.App.UpdateDrive(Number($('#edit-drive-id').value), $('#edit-display-name').value, $('#edit-inventory-number').value, $('#edit-manufacturer').value, $('#edit-device-type').value, $('#edit-storage-location').value, $('#edit-drive-note').value, parseTags($('#edit-drive-tags').value));
    $('#drive-save-status').textContent = 'Gespeichert ✓';
    await Promise.all([loadDrives(), loadInfo()]);
    setTimeout(() => $('#drive-dialog').close(), 350);
  } catch (error) {
    $('#drive-save-status').textContent = `Fehler: ${error}`;
  } finally { button.disabled = false; }
}

async function loadStorageLocations(selected = '') {
  const locations = await window.go.main.App.GetStorageLocations();
  const select = $('#edit-storage-location');
  select.replaceChildren(new Option('Nicht festgelegt', ''));
  locations.forEach((location) => select.add(new Option(location, location)));
  select.value = selected;
}

async function addStorageLocation() {
  const name = prompt('Neuen Lagerort eingeben, z. B. Schrank A / Fach 3:');
  if (!name?.trim()) return;
  try {
    await window.go.main.App.AddStorageLocation(name);
    await loadStorageLocations(name.trim());
  } catch (error) { $('#drive-save-status').textContent = `Fehler: ${error}`; }
}

async function openFileDialog(file) {
  if (file.id && file.metadata === undefined) {
    try { file = await window.go.main.App.GetFileDetails(file.id); } catch (_) { /* Basisdaten weiter anzeigen. */ }
  }
  $('#file-dialog-title').textContent = file.filename;
  $('#detail-drive').textContent = file.drive;
  $('#detail-path').textContent = file.path;
  const dimensions = file.width && file.height ? ` · ${file.width.toLocaleString('de-DE')} × ${file.height.toLocaleString('de-DE')} px` : '';
  $('#detail-type').textContent = `${file.mimeType || (file.extension ? `.${file.extension}` : 'Unbekannt')}${dimensions}`;
  $('#detail-size').textContent = formatBytes(file.size);
  $('#detail-modified').textContent = formatDate(file.modified);
  let metadata = {};
  try { metadata = file.metadata ? JSON.parse(file.metadata) : {}; } catch (_) { metadata = {}; }
  const orientationNames = {1: 'Normal', 2: 'Horizontal gespiegelt', 3: '180°', 4: 'Vertikal gespiegelt', 5: '90° gespiegelt', 6: '90° im Uhrzeigersinn', 7: '270° gespiegelt', 8: '270° im Uhrzeigersinn'};
  const captured = metadata.capturedAt ? metadata.capturedAt.replace(/^(\d{4}):(\d{2}):(\d{2})/, '$3.$2.$1') : '';
  $('#detail-captured').textContent = captured;
  $('#detail-camera').textContent = [metadata.manufacturer, metadata.camera].filter(Boolean).join(' ');
  $('#detail-lens').textContent = metadata.lens || '';
  $('#detail-orientation').textContent = orientationNames[metadata.orientation] || '';
  document.querySelectorAll('.exif-detail').forEach((row) => row.classList.toggle('hidden', !row.querySelector('dd').textContent));
  const previewWrap = $('#preview-wrap');
  const preview = $('#file-preview');
  const documentPreview = $('#document-preview');
  let videoPreview = $('#video-preview');
  if (!videoPreview) {
    videoPreview = document.createElement('video');
    videoPreview.id = 'video-preview';
    videoPreview.className = 'hidden';
    videoPreview.controls = true;
    videoPreview.preload = 'metadata';
    videoPreview.style.maxWidth = '100%';
    videoPreview.style.maxHeight = '420px';
    previewWrap.append(videoPreview);
  }
  const previewStatus = $('#preview-status');
  preview.removeAttribute('src');
  preview.classList.add('hidden');
  documentPreview.removeAttribute('src');
  documentPreview.classList.add('hidden');
  videoPreview.pause();
  videoPreview.removeAttribute('src');
  videoPreview.classList.add('hidden');
  const extension = (file.extension || '').toLowerCase();
  const isPDF = file.mimeType === 'application/pdf' || extension === 'pdf';
  const isVideo = file.mimeType?.startsWith('video/') || ['mp4', 'm4v', 'webm', 'ogv', 'ogg', 'mov'].includes(extension);
  const previewable = file.id && (isPDF || isVideo || file.mimeType?.startsWith('image/') || ['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic', 'heif'].includes(extension));
  previewWrap.classList.toggle('hidden', !previewable);
  if (previewable) {
    previewStatus.classList.remove('hidden');
    previewStatus.textContent = 'Vorschau wird erzeugt …';
    window.go.main.App.GetImagePreview(file.id).then((dataURL) => {
      if (!$('#file-dialog').open) return;
      if (isVideo) {
        videoPreview.src = dataURL;
        videoPreview.classList.remove('hidden');
      } else if (isPDF) {
        documentPreview.src = dataURL;
        documentPreview.style.width = '100%';
        documentPreview.style.height = '420px';
        documentPreview.style.border = '0';
        documentPreview.classList.remove('hidden');
      } else {
        preview.src = dataURL;
        preview.classList.remove('hidden');
      }
      previewStatus.classList.add('hidden');
    }).catch((error) => { previewStatus.textContent = `Keine Vorschau: ${error}`; });
  }
  $('#file-dialog').showModal();
}

async function findDuplicates() {
  const button = $('#duplicate-button');
  button.disabled = true;
  $('#duplicate-status').textContent = 'Dateien gleicher Größe werden per SHA-256 geprüft …';
  $('#duplicate-results').replaceChildren();
  ensureDuplicateControls();
  $('#duplicate-dialog').showModal();
  try {
    const result = await window.go.main.App.FindDuplicates();
    const duplicateFiles = result.groups.reduce((sum, group) => sum + group.files.length, 0);
    $('#duplicate-status').textContent = result.groups.length
      ? `${result.groups.length.toLocaleString('de-DE')} Gruppen mit ${duplicateFiles.toLocaleString('de-DE')} Dateien gefunden${result.skipped ? ` · ${result.skipped} nicht erreichbar` : ''}.`
      : `Keine inhaltlich identischen Dateien gefunden${result.skipped ? ` · ${result.skipped} nicht erreichbar` : ''}.`;
    duplicateGroups = result.groups;
    duplicatePage = 1;
    renderDuplicateGroups();
  } catch (error) {
    $('#duplicate-status').textContent = `Duplikatprüfung fehlgeschlagen: ${error}`;
  } finally {
    button.disabled = false;
  }
}

function ensureDuplicateControls() {
  if ($('#duplicate-filter')) return;
  const toolbar = document.createElement('div');
  toolbar.className = 'duplicate-toolbar';
  const filter = document.createElement('input');
  filter.id = 'duplicate-filter';
  filter.type = 'search';
  filter.placeholder = 'Datenträger oder Pfad filtern …';
  const pageLabel = document.createElement('span');
  pageLabel.id = 'duplicate-page-label';
  toolbar.append(filter, pageLabel);
  $('#duplicate-results').before(toolbar);
  const pagination = document.createElement('div');
  pagination.className = 'duplicate-pagination';
  const previous = document.createElement('button');
  previous.type = 'button';
  previous.className = 'secondary';
  previous.textContent = '← Zurück';
  previous.addEventListener('click', () => { duplicatePage--; renderDuplicateGroups(); });
  const next = document.createElement('button');
  next.type = 'button';
  next.className = 'secondary';
  next.textContent = 'Weiter →';
  next.addEventListener('click', () => { duplicatePage++; renderDuplicateGroups(); });
  pagination.append(previous, next);
  pagination.dataset.role = 'duplicate-pagination';
  $('#duplicate-results').after(pagination);
  filter.addEventListener('input', () => { duplicatePage = 1; renderDuplicateGroups(); });
}

function renderDuplicateGroups() {
  const query = ($('#duplicate-filter')?.value || '').trim().toLocaleLowerCase('de');
  const groups = query ? duplicateGroups.filter((group) => group.files.some((file) => `${file.drive} ${file.path}`.toLocaleLowerCase('de').includes(query))) : duplicateGroups;
  const pages = Math.max(1, Math.ceil(groups.length / duplicatePageSize));
  duplicatePage = Math.max(1, Math.min(duplicatePage, pages));
  const container = $('#duplicate-results');
  container.replaceChildren();
  for (const group of groups.slice((duplicatePage - 1) * duplicatePageSize, duplicatePage * duplicatePageSize)) {
    const card = document.createElement('section');
    card.className = 'duplicate-group';
    const heading = document.createElement('div');
    heading.className = 'duplicate-group-head';
    const title = document.createElement('strong');
    title.textContent = `${group.files.length} identische Dateien`;
    const meta = document.createElement('span');
    meta.textContent = `${formatBytes(group.size)} je Datei · ${formatBytes(group.size * (group.files.length - 1))} zusätzlich belegt`;
    heading.append(title, meta);
    card.append(heading);
    for (const file of group.files) {
      const entry = document.createElement('button');
      entry.type = 'button';
      entry.className = 'secondary duplicate-file';
      const drive = document.createElement('span');
      drive.textContent = file.drive;
      const path = document.createElement('span');
      path.textContent = file.path;
      path.title = file.path;
      entry.append(drive, path);
      entry.addEventListener('click', () => openFileDialog({id: file.id, filename: file.filename, drive: file.drive, path: file.path, size: group.size, extension: '', mimeType: '', modified: ''}));
      card.append(entry);
    }
    container.append(card);
  }
  if (!groups.length) {
    const empty = document.createElement('div');
    empty.className = 'library-empty';
    empty.textContent = 'Keine Duplikatgruppe passt zum Filter.';
    container.append(empty);
  }
  $('#duplicate-page-label').textContent = `${groups.length.toLocaleString('de-DE')} Gruppen · Seite ${duplicatePage} von ${pages}`;
  const pagination = document.querySelector('[data-role="duplicate-pagination"]');
  pagination.children[0].disabled = duplicatePage <= 1;
  pagination.children[1].disabled = duplicatePage >= pages;
}

window.runtime.EventsOn('scan:progress', (event) => {
  $('#scan-title').textContent = event.phase === 'save' ? 'Katalog wird gespeichert …' : `${event.files.toLocaleString('de-DE')} Dateien gefunden`;
  $('#scan-detail').textContent = event.path;
});
window.runtime.EventsOn('duplicates:progress', (event) => {
  $('#duplicate-status').textContent = `${event.done.toLocaleString('de-DE')} von ${event.total.toLocaleString('de-DE')} Kandidaten geprüft …`;
});
scanButton.addEventListener('click', startScan);
$('#nav-overview').addEventListener('click', showOverview);
$('#nav-library').addEventListener('click', showLibrary);
$('#nav-drives').addEventListener('click', showDrives);
$('#nav-archive').addEventListener('click', showArchive);
$('#nav-settings').addEventListener('click', showSettings);
$('#save-settings-button').addEventListener('click', saveSettings);
$('#create-backup-button').addEventListener('click', createBackup);
['#setting-backup-enabled', '#setting-backup-file-unlimited', '#setting-backup-unlimited', '#setting-image-analysis-enabled', '#setting-image-header-unlimited', '#setting-image-scan-unlimited', '#setting-exif-enabled', '#setting-exif-file-unlimited', '#setting-exif-total-unlimited', '#setting-text-enabled', '#setting-text-file-unlimited', '#setting-text-total-unlimited', '#setting-image-preview-enabled', '#setting-image-preview-unlimited', '#setting-thumbnail-cache-unlimited'].forEach((selector) => {
  $(selector).addEventListener('change', syncSettingsControls);
});
$('#drive-scan-button').addEventListener('click', startScan);
$('#refresh-volumes-button').addEventListener('click', async () => {
  const button = $('#refresh-volumes-button');
  button.disabled = true;
  try { await loadDrives(); } finally { button.disabled = false; }
});
$('#search-button').addEventListener('click', () => loadLibrary(1));
$('#search-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadLibrary(1); });
$('#extension-filter').addEventListener('change', () => loadLibrary(1));
$('#drive-filter').addEventListener('change', () => loadLibrary(1));
$('#content-search').addEventListener('change', () => loadLibrary(1));
$('#previous-page').addEventListener('click', () => loadLibrary(libraryPage - 1));
$('#next-page').addEventListener('click', () => loadLibrary(libraryPage + 1));
$('#duplicate-button').addEventListener('click', findDuplicates);
$('#save-drive-button').addEventListener('click', saveDrive);
$('#add-location-button').addEventListener('click', addStorageLocation);
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
