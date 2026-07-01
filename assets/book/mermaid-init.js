// Source: https://github.com/rust-lang/mdBook/issues/762#issuecomment-728161584
// Original Licensce: Unknown, assuming Mozilla Public License 2.0, same as mdbook

// mdBook creates <code> elements with the class 'language-mermaid hljs' whenever you
// define a mermaid code block.
// The mermaid javascript parser looks for elements with a class name 'mermaid'.
// So simply change the class name of the elements to 'mermaid' to make everything work.
function patchMermaidCodeElementClass() {
    var elements = document.getElementsByClassName("language-mermaid");
    Array.from(elements).forEach(element => {
        if (element.tagName.toLowerCase() == "code") {
            element.className = "mermaid";
        }
    });
    
}

patchMermaidCodeElementClass();
mermaid.initialize({startOnLoad:true});
