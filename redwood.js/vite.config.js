import path from 'path'

export default {
  build: {
    lib: {
      formats: ['cjs', 'es'],
      entry: 'index.js',
      name: 'redwood.js',
    },
  }
}
