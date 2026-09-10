#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="lyrics-display"
VERSION="${VERSION:-dev}"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo local)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}"

DIST="$ROOT/dist"
APP="$DIST/${APP_NAME}.app"
MAKE_DMG=0

if [[ "${1:-}" == "--dmg" ]]; then
  MAKE_DMG=1
fi

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "==> 编译 ${APP_NAME} (${VERSION})"
(
  cd "$ROOT"
  CGO_ENABLED=1 go build -ldflags "$LDFLAGS" -o "$APP/Contents/MacOS/${APP_NAME}" .
)

echo "==> 写入 Info.plist"
sed "s/__VERSION__/${VERSION}/g" "$ROOT/packaging/Info.plist" > "$APP/Contents/Info.plist"

if [[ -f "$ROOT/packaging/AppIcon.icns" ]]; then
  echo "==> 复制 AppIcon.icns"
  cp "$ROOT/packaging/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
else
  ICON_SRC=""
  for candidate in "$ROOT/packaging/icon.png" "$ROOT/packaging/icon.jpg"; do
    if [[ -f "$candidate" ]]; then
      ICON_SRC="$candidate"
      break
    fi
  done

  if [[ -n "$ICON_SRC" ]]; then
    echo "==> 生成 AppIcon.icns"
    ICONSET="$DIST/AppIcon.iconset"
    rm -rf "$ICONSET"
    mkdir -p "$ICONSET"
    for size in 16 32 128 256 512; do
      sips -z "$size" "$size" "$ICON_SRC" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
      sips -z $((size * 2)) $((size * 2)) "$ICON_SRC" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
    done
    if iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns" 2>/dev/null; then
      rm -rf "$ICONSET"
    else
      echo "iconutil 不可用，改用 sips 生成 icns"
      sips -s format icns "$ICON_SRC" --out "$APP/Contents/Resources/AppIcon.icns" >/dev/null
      rm -rf "$ICONSET"
    fi
  else
    echo "==> 未找到 packaging/AppIcon.icns，跳过图标"
  fi
fi

echo "==> 签名"
codesign --force --deep --sign - --timestamp=none "$APP" >/dev/null

echo "==> 完成: $APP"

if [[ "$MAKE_DMG" -eq 1 ]]; then
  STAGE="$DIST/dmg-stage"
  DMG="$DIST/${APP_NAME}-${VERSION}.dmg"
  rm -rf "$STAGE" "$DMG"
  mkdir -p "$STAGE"
  cp -R "$APP" "$STAGE/${APP_NAME}.app"
  ln -sf /Applications "$STAGE/Applications"

  echo "==> 制作 DMG"
  if hdiutil create -volname "$APP_NAME" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null; then
    echo "==> 完成: $DMG"
  else
    echo "hdiutil create 失败，.app 仍可直接使用: $APP" >&2
    exit 1
  fi
  rm -rf "$STAGE"
fi
