// Plugins
import vue from '@vitejs/plugin-vue'
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'

// Utilities
import { defineConfig, loadEnv, type Plugin } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import { randomBytes } from 'crypto'
import { readFileSync } from 'node:fs'
import postcss from 'postcss'

// The panel reads version.json to tell whether the installed UI matches it.
const appVersion: string = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')).version

function emitVersionFile(): Plugin {
  return {
    name: 'emit-version-file',
    generateBundle() {
      this.emitFile({ type: 'asset', fileName: 'version.json', source: JSON.stringify({ version: appVersion }) + '\n' })
    },
  }
}

function getUniqueFileName(template) {
  if (template.includes('.js') || template.includes('.css')) {
    const hash = randomBytes(8).toString('hex')
    return template.replace('[name]', hash)
  }
  return template
}

function preserveStandardBackdropFilter(): Plugin {
  return {
    name: 'preserve-standard-backdrop-filter',
    enforce: 'post' as const,
    generateBundle(_options, bundle) {
      for (const asset of Object.values(bundle)) {
        if (asset.type !== 'asset' || !asset.fileName.endsWith('.css')) continue
        const source = typeof asset.source === 'string'
          ? asset.source
          : Buffer.from(asset.source).toString('utf8')
        const root = postcss.parse(source)
        root.walkDecls('-webkit-backdrop-filter', declaration => {
          const previous = declaration.prev()
          if (previous?.type === 'decl' &&
              previous.prop === 'backdrop-filter' &&
              previous.value === declaration.value) return
          declaration.cloneBefore({ prop: 'backdrop-filter' })
        })
        asset.source = root.toString()
      }
    },
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const proxyTarget = env.VITE_DEV_PROXY_TARGET?.trim() || 'http://127.0.0.1:2097'

  return {
    base: './',
    plugins: [
      vue({
        template: { transformAssetUrls },
      }),
      vuetify({
        autoImport: true,
        styles: {
          configFile: 'src/styles/settings.scss',
        },
      }),
      preserveStandardBackdropFilter(),
      emitVersionFile(),
    ],
    build: {
      manifest: false,
      outDir: 'dist',
      chunkSizeWarningLimit: 2000,
      rollupOptions: {
        output: {
          // Each page loads its own chunk, so the first visit downloads far
          // less. main.ts reloads a tab whose chunks a UI update replaced.
          codeSplitting: true,
          entryFileNames: getUniqueFileName('assets/[name].js'),
          chunkFileNames: getUniqueFileName('assets/[name].js'),
          assetFileNames: (assetInfo) => {
            if (assetInfo.names.some(name => name.endsWith('.css')))
              return getUniqueFileName('assets/[name].css')
            return 'assets/' + assetInfo.names[0]
          },
        },
      },
    },
    define: { 'process.env': {}, __APP_VERSION__: JSON.stringify(appVersion) },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
      extensions: ['.js', '.json', '.jsx', '.mjs', '.ts', '.tsx', '.vue'],
    },
    server: {
      port: 3000,
      proxy: {
		'/api': {
		  target: proxyTarget,
		  ws: true,
		},
		'/apiv2': {
		  target: proxyTarget,
		  ws: true,
		},
		'/agent/v1': {
		  target: proxyTarget,
		  ws: true,
        },
      },
    }
  }
})
