(function() {
    function addBackToWebsiteLink() {
        var chapterList = document.querySelector(".sidebar-scrollbox ol.chapter");
        if (chapterList) {
            if (document.getElementById("back-to-website-link")) {
                return;
            }
            var li = document.createElement("li");
            li.id = "back-to-website-link";
            li.className = "chapter-item expanded affix";
            
            var a = document.createElement("a");
            a.href = "../index.html";
            a.innerHTML = "<strong>← Back to Website</strong>";
            
            // Style formatting
            a.style.display = "block";
            a.style.padding = "10px 0";
            
            li.appendChild(a);
            chapterList.insertBefore(li, chapterList.firstChild);
        } else {
            setTimeout(addBackToWebsiteLink, 50);
        }
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", addBackToWebsiteLink);
    } else {
        addBackToWebsiteLink();
    }
})();
