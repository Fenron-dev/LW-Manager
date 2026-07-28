# VaultApp-Markenassets

Das Logo verbindet das „V“ von VaultApp mit der kreisförmigen Anmutung eines
Datenträgers beziehungsweise einer Tresortür.

## Farben

- Navy: `#0d121c`
- Mint: `#55d6be`
- Helle Wortmarke: `#eaf0f7`

## Varianten

- `vaultapp-mark-master.png`: transparente 1024-Pixel-Masterdatei
- `vaultapp-mark-{16…1024}.png`: quadratische PNG-Größen
- `vaultapp-mark-on-navy.png`: Vorschau auf dem dunklen App-Hintergrund
- `vaultapp-logo-light.png`: horizontale Wortmarke für helle Flächen
- `vaultapp-logo-dark.png`: horizontale Wortmarke für dunkle Flächen
- `../icons/windows/VaultApp.ico`: Windows-Icon mit mehreren Auflösungen
- `../icons/macos/VaultApp.icns`: macOS-Icon
- `../icons/macos/VaultApp.iconset/`: alle macOS-Ausgangsgrößen
- `../icons/linux/hicolor/`: Linux-Hicolor-Struktur von 16 bis 512 Pixel

Die von Wails verwendeten Kopien liegen zusätzlich unter `build/`. Die
Weboberfläche verwendet optimierte Kopien unter `frontend/dist/assets/`.

Alle Varianten können mit `scripts/generate-logo-assets.py` reproduzierbar aus
der transparenten Masterdatei erzeugt werden. Das Skript benötigt Pillow.
