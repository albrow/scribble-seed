(function() {
	var el = document.getElementById("nav");
	el.onclick = function() {
		if (el.classList) {
			if (el.classList.contains("close")) {
				el.classList.remove("close");
			} else {
				el.classList.add("close");
			}
		} else {
			el.className += ' ' + "close";
		}
	};
})();
