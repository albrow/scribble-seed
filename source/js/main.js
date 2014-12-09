(function() {
	var nav = document.getElementById("nav");
	var menu = document.getElementById("nav-menu");
	nav.onclick = function() {
		toggleNav(nav, menu);
	}

	function toggleNav(nav, menu) {
		toggleClass(nav, "close");
		toggleClass(menu, "show");
	}

	function toggleClass(el, name) {
		if (el.classList) {
			if (el.classList.contains(name)) {
				el.classList.remove(name);
			} else {
				el.classList.add(name);
			}
		} else {
			el.className += ' ' + name;
		}
	}
})();
