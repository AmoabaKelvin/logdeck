// Turbopack loader: `import body from "./page.md"` gives the file's text.
// Keeps the markdown in the bundle, so nothing reads the disk at runtime.
module.exports = function mdLoader(source) {
  return `export default ${JSON.stringify(source)};`;
};
