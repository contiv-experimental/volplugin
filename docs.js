require('mdoc').run({
  // configuration options (specified below)
  inputDir: 'docs',
  outputDir: 'dist',
	indexContentPath: "docs/1_index.md",
  exclude: '.*,*.go',
	baseTitle: 'volplugin Documentation',
  mapTocName: function(filename, tocObj, title) {
    strings = filename.split('_')
    return strings[0] + ". " + title
  },
  parsingFunction: require('marked')
})
