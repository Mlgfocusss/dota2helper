interface ImportMetaEnv {
  readonly VITE_APP_NAME: string
  readonly VITE_API_BASE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
