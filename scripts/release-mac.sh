#!/bin/bash
set -euo pipefail

# Print steps with nice colors
info() {
  echo -e "\033[1;34m[INFO]\033[0m $1"
}

success() {
  echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

error() {
  echo -e "\033[1;31m[ERROR]\033[0m $1" >&2
}

warn() {
  echo -e "\033[1;33m[WARNING]\033[0m $1"
}

# Check for required tools
for tool in wails codesign pkgbuild productsign xcrun install_name_tool otool; do
  if ! command -v "$tool" &> /dev/null; then
    error "$tool is required but not installed. Exiting."
    exit 1
  fi
done

info "Starting macOS Release Build and Packaging Process for twnoaa"

# Resolve version from Git tag and commit hash
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_VERSION="${VERSION}(${COMMIT})"
info "Build version: ${BUILD_VERSION}"

# Target macOS platform (default: darwin/arm64 for Apple Silicon)
PLATFORM=${MACOS_PLATFORM:-"darwin/arm64"}
info "Target platform: ${PLATFORM}"

# Step 1: Build the frontend explicitly
info "Building frontend assets..."
(cd frontend && npm install && npm run build)
success "Frontend built successfully."

# Step 2: Build the application using Wails
info "Building Wails application for macOS (${PLATFORM})..."
wails build -platform "${PLATFORM}" -clean -ldflags "-X main.version=${BUILD_VERSION}"

ORIG_APP_PATH="build/bin/twnoaa.app"
if [ ! -d "$ORIG_APP_PATH" ]; then
  error "Build failed: $ORIG_APP_PATH not found."
  exit 1
fi

mkdir -p dist
APP_PATH="dist/twnoaa.app"
rm -rf "$APP_PATH"
mv "$ORIG_APP_PATH" "$APP_PATH"

success "Application built and prepared successfully at $APP_PATH"

# Step 2.5: Bundle dynamic libraries (librtlsdr, libusb) into Frameworks
info "Bundling dynamic libraries into $APP_PATH/Contents/Frameworks..."
FRAMEWORKS_DIR="$APP_PATH/Contents/Frameworks"
mkdir -p "$FRAMEWORKS_DIR"

LIB_SEARCH_PATHS=(
  "/opt/homebrew/opt/librtlsdr/lib"
  "/opt/homebrew/lib"
  "/usr/local/opt/librtlsdr/lib"
  "/usr/local/lib"
)

RTLSDR_LIB=""
for dir in "${LIB_SEARCH_PATHS[@]}"; do
  if [ -f "$dir/librtlsdr.0.dylib" ]; then
    RTLSDR_LIB="$dir/librtlsdr.0.dylib"
    break
  elif [ -f "$dir/librtlsdr.dylib" ]; then
    RTLSDR_LIB="$dir/librtlsdr.dylib"
    break
  fi
done

LIBUSB_SEARCH_PATHS=(
  "/opt/homebrew/opt/libusb/lib"
  "/opt/homebrew/lib"
  "/usr/local/opt/libusb/lib"
  "/usr/local/lib"
)

LIBUSB_LIB=""
for dir in "${LIBUSB_SEARCH_PATHS[@]}"; do
  if [ -f "$dir/libusb-1.0.0.dylib" ]; then
    LIBUSB_LIB="$dir/libusb-1.0.0.dylib"
    break
  elif [ -f "$dir/libusb-1.0.dylib" ]; then
    LIBUSB_LIB="$dir/libusb-1.0.dylib"
    break
  fi
done

if [ -z "$RTLSDR_LIB" ]; then
  error "librtlsdr not found in Homebrew or /usr/local. Please install it first (brew install librtlsdr)."
  exit 1
fi

if [ -z "$LIBUSB_LIB" ]; then
  error "libusb not found in Homebrew or /usr/local. Please install it first (brew install libusb)."
  exit 1
fi

info "Found librtlsdr at: $RTLSDR_LIB"
info "Found libusb at: $LIBUSB_LIB"

# Copy library files and dereference symlinks
cp -L "$RTLSDR_LIB" "$FRAMEWORKS_DIR/librtlsdr.0.dylib"
cp -L "$LIBUSB_LIB" "$FRAMEWORKS_DIR/libusb-1.0.0.dylib"
chmod 755 "$FRAMEWORKS_DIR"/*.dylib

# Create symlinks inside Frameworks for compatibility
(cd "$FRAMEWORKS_DIR" && ln -sf librtlsdr.0.dylib librtlsdr.dylib)
(cd "$FRAMEWORKS_DIR" && ln -sf libusb-1.0.0.dylib libusb-1.0.dylib)

# Update install names for libusb
install_name_tool -id "@rpath/libusb-1.0.0.dylib" "$FRAMEWORKS_DIR/libusb-1.0.0.dylib"

# Update install names and dependency links for librtlsdr
install_name_tool -id "@rpath/librtlsdr.0.dylib" "$FRAMEWORKS_DIR/librtlsdr.0.dylib"
LIBUSB_REF=$(otool -L "$FRAMEWORKS_DIR/librtlsdr.0.dylib" | grep "libusb-1.0" | awk '{print $1}')
if [ -n "$LIBUSB_REF" ]; then
  install_name_tool -change "$LIBUSB_REF" "@rpath/libusb-1.0.0.dylib" "$FRAMEWORKS_DIR/librtlsdr.0.dylib"
fi

# Update main executable
MAIN_BIN="$APP_PATH/Contents/MacOS/twnoaa"
RTLSDR_REF=$(otool -L "$MAIN_BIN" | grep "librtlsdr" | awk '{print $1}')
if [ -n "$RTLSDR_REF" ]; then
  install_name_tool -change "$RTLSDR_REF" "@rpath/librtlsdr.0.dylib" "$MAIN_BIN"
fi

# Add @executable_path/../Frameworks to rpath if not already present
if ! otool -l "$MAIN_BIN" | grep -q "@executable_path/../Frameworks"; then
  install_name_tool -add_rpath "@executable_path/../Frameworks" "$MAIN_BIN"
fi

success "Dynamic libraries bundled and paths configured successfully."

# Step 3: Select/Determine Application Signing Identity
APP_IDENTITY=${MACOS_APP_IDENTITY:-""}

if [ -z "$APP_IDENTITY" ]; then
  info "MACOS_APP_IDENTITY is not set. Scanning Keychain for valid 'Developer ID Application' certificates..."
  
  identities=$(security find-identity -v)
  app_certs=()
  
  while IFS= read -r line; do
    if [[ "$line" =~ \"(Developer\ ID\ Application:[^\"]+)\" ]]; then
      app_certs+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$identities"

  if [ ${#app_certs[@]} -eq 0 ]; then
    error "No valid 'Developer ID Application' certificates found in your Keychain."
    error "Please set MACOS_APP_IDENTITY environment variable or import a valid certificate."
    exit 1
  elif [ ${#app_certs[@]} -eq 1 ]; then
    APP_IDENTITY="${app_certs[0]}"
    info "Found exactly one certificate. Automatically selected: $APP_IDENTITY"
  else
    info "Multiple certificates found. Please select one:"
    for i in "${!app_certs[@]}"; do
      echo "  $((i+1))) ${app_certs[i]}"
    done
    
    while true; do
      read -p "Select certificate number (1-${#app_certs[@]}): " cert_idx < /dev/tty
      if [[ "$cert_idx" =~ ^[0-9]+$ ]] && [ "$cert_idx" -ge 1 ] && [ "$cert_idx" -le "${#app_certs[@]}" ]; then
        APP_IDENTITY="${app_certs[$((cert_idx-1))]}"
        break
      else
        warn "Invalid selection. Please enter a number between 1 and ${#app_certs[@]}."
      fi
    done
  fi
fi

info "Selected Application Identity: $APP_IDENTITY"

# Step 4: Sign the bundled libraries and .app bundle with Hardened Runtime
info "Signing bundled libraries with hardened runtime..."
for lib in "$FRAMEWORKS_DIR"/*.dylib; do
  if [ -f "$lib" ] && [ ! -L "$lib" ]; then
    codesign --force --options runtime --timestamp --sign "$APP_IDENTITY" "$lib"
  fi
done

info "Signing the application bundle with hardened runtime..."
codesign --force --options runtime --timestamp --sign "$APP_IDENTITY" --deep "$APP_PATH"
success "Application bundle and bundled libraries signed successfully."

# Step 5: Select/Determine Installer Signing Identity
PKG_IDENTITY=${MACOS_PKG_IDENTITY:-""}

if [ -z "$PKG_IDENTITY" ]; then
  info "MACOS_PKG_IDENTITY is not set. Scanning Keychain for valid 'Developer ID Installer' certificates..."
  
  identities=$(security find-identity -v)
  pkg_certs=()
  
  while IFS= read -r line; do
    if [[ "$line" =~ \"(Developer\ ID\ Installer:[^\"]+)\" ]]; then
      pkg_certs+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$identities"

  if [ ${#pkg_certs[@]} -eq 0 ]; then
    error "No valid 'Developer ID Installer' certificates found in your Keychain."
    error "Please set MACOS_PKG_IDENTITY environment variable or import a valid certificate."
    exit 1
  elif [ ${#pkg_certs[@]} -eq 1 ]; then
    PKG_IDENTITY="${pkg_certs[0]}"
    info "Found exactly one installer certificate. Automatically selected: $PKG_IDENTITY"
  else
    info "Multiple installer certificates found. Please select one:"
    for i in "${!pkg_certs[@]}"; do
      echo "  $((i+1))) ${pkg_certs[i]}"
    done
    
    while true; do
      read -p "Select installer certificate number (1-${#pkg_certs[@]}): " cert_idx < /dev/tty
      if [[ "$cert_idx" =~ ^[0-9]+$ ]] && [ "$cert_idx" -ge 1 ] && [ "$cert_idx" -le "${#pkg_certs[@]}" ]; then
        PKG_IDENTITY="${pkg_certs[$((cert_idx-1))]}"
        break
      else
        warn "Invalid selection. Please enter a number between 1 and ${#pkg_certs[@]}."
      fi
    done
  fi
fi

info "Selected Installer Identity: $PKG_IDENTITY"

# Step 6: Package the signed .app into an unsigned .pkg (disabling relocation)
ARCH_SUFFIX=$(echo "$PLATFORM" | cut -d'/' -f2)
UNSIGNED_PKG="dist/twnoaa-unsigned.pkg"
SIGNED_PKG="dist/twnoaa-${VERSION}-darwin-${ARCH_SUFFIX}.pkg"
COMPONENT_PLIST="dist/components.plist"

info "Creating the installer package..."
pkgbuild --analyze --root "dist" "$COMPONENT_PLIST"
plutil -replace 0.BundleIsRelocatable -bool NO "$COMPONENT_PLIST"

pkgbuild --root "dist" --component-plist "$COMPONENT_PLIST" --install-location "/Applications" "$UNSIGNED_PKG"
rm -f "$COMPONENT_PLIST"
success "Installer package created at $UNSIGNED_PKG"

# Step 7: Sign the installer package
info "Signing the installer package..."
productsign --sign "$PKG_IDENTITY" "$UNSIGNED_PKG" "$SIGNED_PKG"
rm -f "$UNSIGNED_PKG"
cp -f "$SIGNED_PKG" "dist/twnoaa.pkg"
success "Signed installer package created at $SIGNED_PKG (and dist/twnoaa.pkg)"

# Step 8: Notarize the package
NOTARY_PROFILE=${MACOS_NOTARY_PROFILE:-""}

if [ -z "$NOTARY_PROFILE" ]; then
  info "MACOS_NOTARY_PROFILE environment variable is not set."
  read -p "Enter Apple Notarization profile name (or press Enter to exit/skip notarization): " NOTARY_PROFILE < /dev/tty
  if [ -z "$NOTARY_PROFILE" ]; then
    error "Apple Notarization profile is required to complete notarization. Skipping notarization."
    info "You can notarize manually or rerun after setting MACOS_NOTARY_PROFILE environment variable."
    exit 0
  fi
fi

info "Submitting the signed package to Apple for notarization using profile: $NOTARY_PROFILE..."
xcrun notarytool submit "$SIGNED_PKG" --keychain-profile "$NOTARY_PROFILE" --wait

info "Stapling notarization ticket to the installer package..."
xcrun stapler staple "$SIGNED_PKG"
if [ -f "dist/twnoaa.pkg" ]; then
  xcrun stapler staple "dist/twnoaa.pkg" || true
fi

success "macOS packaging and notarization completed successfully! Signed and notarized installer is at: $SIGNED_PKG"
