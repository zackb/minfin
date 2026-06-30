#!/usr/bin/env bash
# Regenerate every app icon from the single source (assets/icon.png):
#   - GTK hicolor PNGs (committed, so `make install-gtk` needs no ImageMagick)
#   - iOS AppIcon.appiconset (the iOS app's asset dir, created ahead of the app)
# Run this whenever assets/icon.png changes.
set -euo pipefail
cd "$(dirname "$0")/.."

SOURCE="assets/icon.png"
APPID="com.zackbartel.minfin"
CONVERT="$(command -v magick || command -v convert || true)"

[ -n "$CONVERT" ] || { echo "ImageMagick (magick/convert) required"; exit 1; }
[ -f "$SOURCE" ] || { echo "missing $SOURCE"; exit 1; }

echo "GTK hicolor icons…"
for size in 16 24 32 48 64 128 256 512; do
	dir="assets/icons/hicolor/${size}x${size}/apps"
	mkdir -p "$dir"
	"$CONVERT" "$SOURCE" -resize "${size}x${size}" "$dir/${APPID}.png"
done

echo "iOS AppIcon…"
IOS="apple/minfin/Assets.xcassets/AppIcon.appiconset"
mkdir -p "$IOS"
"$CONVERT" "$SOURCE" -resize 1024x1024 "$IOS/Image.png"
# ponytail: single 1024² universal icon — Xcode derives the device sizes.
cat > "$IOS/Contents.json" <<'EOF'
{
  "images" : [
    {
      "filename" : "Image.png",
      "idiom" : "universal",
      "platform" : "ios",
      "size" : "1024x1024"
    }
  ],
  "info" : {
    "author" : "minfin",
    "version" : 1
  }
}
EOF

echo "done"
