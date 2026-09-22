# LW-Manager-Markenassets

Das Logo verbindet das Monogramm „LW“ mit der kreisförmigen Anmutung eines
Datenträgers beziehungsweise einer Tresortür.

## Farben

- Navy: `#0d121c`
- Mint: `#55d6be`
- Helle Wortmarke: `#eaf0f7`

## Varianten

- `lw-manager-mark-source.png`: transparentes, bildgeneriertes Ausgangsmotiv
- `lw-manager-mark-master.png`: farbbereinigte 1024-Pixel-Masterdatei
- `lw-manager-mark-{16…1024}.png`: quadratische PNG-Größen
- `lw-manager-mark-on-navy.png`: Vorschau auf dem dunklen App-Hintergrund
- `lw-manager-logo-light.png`: horizontale Wortmarke für helle Flächen
- `lw-manager-logo-dark.png`: horizontale Wortmarke für dunkle Flächen
- `../icons/windows/LW-Manager.ico`: Windows-Icon mit mehreren Auflösungen
- `../icons/macos/LW-Manager.icns`: macOS-Icon
- `../icons/macos/LW-Manager.iconset/`: alle macOS-Ausgangsgrößen
- `../icons/linux/hicolor/`: Linux-Hicolor-Struktur von 16 bis 512 Pixel

Die von Wails verwendeten Kopien liegen zusätzlich unter `build/`. Die
Weboberfläche verwendet optimierte Kopien unter `frontend/dist/assets/`.

Alle Varianten können mit `scripts/generate-logo-assets.py` reproduzierbar aus
der transparenten Masterdatei erzeugt werden. Das Skript benötigt Pillow.
