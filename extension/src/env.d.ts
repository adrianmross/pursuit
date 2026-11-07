// esbuild loads these as text (build.mjs).
declare module "*.svg" {
  const text: string
  export default text
}
declare module "*.css" {
  const text: string
  export default text
}
