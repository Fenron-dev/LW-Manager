#!/usr/bin/env python3
"""Generate VaultApp logo variants from the transparent ImageGen master."""

from __future__ import annotations

import shutil
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parents[1]
BRAND = ROOT / "assets" / "brand"
ICONS = ROOT / "assets" / "icons"
WEB = ROOT / "frontend" / "dist" / "assets"
BUILD = ROOT / "build"
SOURCE = BRAND / "vaultapp-mark-master.png"

NAVY = (13, 18, 28, 255)
TEAL = (85, 214, 190, 255)
WHITE = (234, 240, 247, 255)


def brand_color(pixel: tuple[int, int, int, int]) -> tuple[int, int, int, int]:
    r, g, b, a = pixel
    if a == 0:
        return (0, 0, 0, 0)
    navy_distance = sum((value - target) ** 2 for value, target in zip((r, g, b), NAVY))
    teal_distance = sum((value - target) ** 2 for value, target in zip((r, g, b), TEAL))
    color = NAVY if navy_distance <= teal_distance else TEAL
    return color[:3] + (a,)


def prepare_master() -> Image.Image:
    source = Image.open(SOURCE).convert("RGBA")
    flat = Image.new("RGBA", source.size)
    pixels = source.get_flattened_data() if hasattr(source, "get_flattened_data") else source.getdata()
    flat.putdata([brand_color(pixel) for pixel in pixels])
    bbox = flat.getbbox()
    if bbox is None:
        raise RuntimeError("Logo master is empty")
    cropped = flat.crop(bbox)
    cropped.thumbnail((880, 880), Image.Resampling.LANCZOS)
    master = Image.new("RGBA", (1024, 1024))
    master.alpha_composite(cropped, ((1024 - cropped.width) // 2, (1024 - cropped.height) // 2))
    return master


def save_png(image: Image.Image, path: Path, size: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    image.resize((size, size), Image.Resampling.LANCZOS).save(path, "PNG", optimize=True)


def font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for path in (
        Path("/System/Library/Fonts/Supplemental/Arial Bold.ttf"),
        Path("/System/Library/Fonts/SFNS.ttf"),
        Path("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"),
    ):
        if path.exists():
            return ImageFont.truetype(str(path), size=size)
    return ImageFont.load_default()


def save_wordmark(master: Image.Image, path: Path, color: tuple[int, int, int, int]) -> None:
    canvas = Image.new("RGBA", (1600, 420))
    canvas.alpha_composite(master.resize((330, 330), Image.Resampling.LANCZOS), (45, 45))
    ImageDraw.Draw(canvas).text((405, 105), "VaultApp", font=font(184), fill=color, anchor="la")
    canvas.save(path, "PNG", optimize=True)


def generate() -> None:
    for directory in (BRAND, ICONS, WEB, BUILD / "windows", BUILD / "darwin"):
        directory.mkdir(parents=True, exist_ok=True)

    master = prepare_master()
    master.save(SOURCE, "PNG", optimize=True)
    for size in (1024, 512, 256, 180, 128, 64, 48, 32, 24, 16):
        save_png(master, BRAND / f"vaultapp-mark-{size}.png", size)

    save_wordmark(master, BRAND / "vaultapp-logo-dark.png", WHITE)
    save_wordmark(master, BRAND / "vaultapp-logo-light.png", NAVY)
    navy_tile = Image.new("RGBA", (1024, 1024), NAVY)
    navy_tile.alpha_composite(master)
    navy_tile.save(BRAND / "vaultapp-mark-on-navy.png", "PNG", optimize=True)

    shutil.copy2(BRAND / "vaultapp-mark-1024.png", BUILD / "appicon.png")
    shutil.copy2(BRAND / "vaultapp-mark-64.png", WEB / "vaultapp-mark-64.png")
    shutil.copy2(BRAND / "vaultapp-mark-32.png", WEB / "favicon-32.png")
    shutil.copy2(BRAND / "vaultapp-mark-180.png", WEB / "apple-touch-icon.png")

    ico_sizes = [(size, size) for size in (16, 24, 32, 48, 64, 128, 256)]
    windows_icon = BUILD / "windows" / "icon.ico"
    master.save(windows_icon, "ICO", sizes=ico_sizes)
    (ICONS / "windows").mkdir(parents=True, exist_ok=True)
    shutil.copy2(windows_icon, ICONS / "windows" / "VaultApp.ico")

    iconset = ICONS / "macos" / "VaultApp.iconset"
    for filename, size in {
        "icon_16x16.png": 16,
        "icon_16x16@2x.png": 32,
        "icon_32x32.png": 32,
        "icon_32x32@2x.png": 64,
        "icon_128x128.png": 128,
        "icon_128x128@2x.png": 256,
        "icon_256x256.png": 256,
        "icon_256x256@2x.png": 512,
        "icon_512x512.png": 512,
        "icon_512x512@2x.png": 1024,
    }.items():
        save_png(master, iconset / filename, size)
    icns = ICONS / "macos" / "VaultApp.icns"
    master.save(icns, "ICNS")
    shutil.copy2(icns, BUILD / "darwin" / "VaultApp.icns")

    for size in (16, 24, 32, 48, 64, 128, 256, 512):
        save_png(master, ICONS / "linux" / "hicolor" / f"{size}x{size}" / "apps" / "vaultapp.png", size)

    print("Generated VaultApp logo assets.")


if __name__ == "__main__":
    generate()
