#!/bin/bash
set -e

echo "📦 Downloading static assets..."

STATIC_DIR="web/static"
mkdir -p "$STATIC_DIR/css" "$STATIC_DIR/js"

# HTMX
HTMX_VERSION="2.0.7"
echo "⬇️  Downloading HTMX v$HTMX_VERSION..."
curl -L -s "https://unpkg.com/htmx.org@$HTMX_VERSION/dist/htmx.min.js" \
  -o "$STATIC_DIR/js/htmx-$HTMX_VERSION.min.js"

# Verify HTMX downloaded successfully
if [ -f "$STATIC_DIR/js/htmx-$HTMX_VERSION.min.js" ]; then
  HTMX_SIZE=$(wc -c < "$STATIC_DIR/js/htmx-$HTMX_VERSION.min.js")
  echo "   ✅ HTMX downloaded: ${HTMX_SIZE} bytes"
else
  echo "   ❌ HTMX download failed"
  exit 1
fi

# Tailwind CSS (build from source)
TAILWIND_VERSION="4.1.17"
echo "⬇️  Building Tailwind CSS v$TAILWIND_VERSION..."

# Check if npm/node is available
if ! command -v npm &> /dev/null; then
    echo "   ❌ Node.js and npm are required to build Tailwind CSS v$TAILWIND_VERSION"
    echo "   💡 Install Node.js from https://nodejs.org/ or use:"
    echo "      brew install node"
    exit 1
fi

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "   📦 Installing dependencies..."
    npm install
fi

# Build Tailwind CSS
echo "   🔨 Building CSS..."
npm run build-css

# Verify Tailwind was built successfully  
if [ -f "$STATIC_DIR/css/tailwindcss-$TAILWIND_VERSION.min.css" ]; then
  TAILWIND_SIZE=$(wc -c < "$STATIC_DIR/css/tailwindcss-$TAILWIND_VERSION.min.css")
  echo "   ✅ Tailwind CSS built: ${TAILWIND_SIZE} bytes"
else
  echo "   ❌ Tailwind CSS build failed"
  exit 1
fi

# Alpine.js
ALPINE_VERSION="3.15.2"
echo "⬇️  Downloading Alpine.js v$ALPINE_VERSION..."
curl -L -s "https://cdn.jsdelivr.net/npm/alpinejs@$ALPINE_VERSION/dist/cdn.min.js" \
  -o "$STATIC_DIR/js/alpine-$ALPINE_VERSION.min.js"

# Verify Alpine.js downloaded successfully
if [ -f "$STATIC_DIR/js/alpine-$ALPINE_VERSION.min.js" ]; then
  ALPINE_SIZE=$(wc -c < "$STATIC_DIR/js/alpine-$ALPINE_VERSION.min.js")
  echo "   ✅ Alpine.js downloaded: ${ALPINE_SIZE} bytes"
else
  echo "   ❌ Alpine.js download failed"
  exit 1
fi

# Create versions file for reference
cat > "$STATIC_DIR/versions.json" << EOF
{
  "htmx": "$HTMX_VERSION",
  "tailwind": "$TAILWIND_VERSION",
  "alpine": "$ALPINE_VERSION",
  "updated": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

echo "📄 Asset versions recorded in $STATIC_DIR/versions.json"
echo "✅ All assets downloaded successfully!"
echo ""
echo "Next steps:"
echo "1. Update HTML templates to use local assets"
echo "2. Configure static file serving in Go server"
echo "3. Test in development environment"