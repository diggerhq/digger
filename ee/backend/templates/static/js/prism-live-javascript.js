Prism.Live.registerLanguage("clike", {
    comments: {
        singleline: "//",
        multiline: ["/*", "*/"]
    },
    snippets: {
        if: `if ($1) {
	$2
}`
    }
});

Prism.Live.registerLanguage("javascript", {
    snippets: {
        log: "console.log($1)",
    }
}, Prism.Live.languages.clike);
