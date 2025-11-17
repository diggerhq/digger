/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_REMOTE_RUNS_ENABLED?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
