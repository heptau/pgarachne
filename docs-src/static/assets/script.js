(function () {
	var toggle = document.querySelector(".nav-toggle");
	var body = document.body;
	var overlay = document.getElementById("toc-overlay");

	if (toggle) {
		function closeToc() {
			body.classList.remove("toc-open");
			toggle.setAttribute("aria-expanded", "false");
		}

		toggle.addEventListener("click", function () {
			var isOpen = body.classList.toggle("toc-open");
			toggle.setAttribute("aria-expanded", isOpen ? "true" : "false");
		});

		if (overlay) {
			overlay.addEventListener("click", closeToc);
		}
	}

	function parseJSONScript(id, fallback) {
		var el = document.getElementById(id);
		if (!el || !el.textContent) return fallback;
		try {
			return JSON.parse(el.textContent);
		} catch (err) {
			return fallback;
		}
	}

	// The search index is same-origin static JSON, but treat entries as
	// untrusted before writing them into href — reject any URL scheme
	// (javascript:, data:, ...) or protocol-relative URL, so a corrupted or
	// tampered index can't smuggle script execution into the DOM.
	function isSafeRelativeUrl(url) {
		return (
			typeof url === "string" &&
			url !== "" &&
			!url.startsWith("//") &&
			!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)
		);
	}

	function addCopyButtons(texts) {
		var blocks = document.querySelectorAll("pre code");
		blocks.forEach(function (code) {
			var pre = code.parentElement;
			if (!pre || pre.querySelector(".btn-code-copy")) return;

			var btn = document.createElement("button");
			btn.type = "button";
			btn.className = "btn-code-copy";
			btn.textContent = texts.copy || "Copy";
			btn.setAttribute("aria-label", texts.copy || "Copy");
			pre.classList.add("code-block-wrap");
			pre.appendChild(btn);

			btn.addEventListener("click", function () {
				var text = code.innerText;
				navigator.clipboard.writeText(text).then(function () {
					var original = btn.textContent;
					btn.textContent = texts.copied || "Copied";
					btn.classList.add("copied");
					setTimeout(function () {
						btn.textContent = original;
						btn.classList.remove("copied");
					}, 1400);
				});
			});
		});
	}

	function initSearch(data, texts) {
		var input = document.getElementById("site-search-input");
		var results = document.getElementById("site-search-results");
		if (!input || !results || !Array.isArray(data) || !data.length) return;

		function hide() {
			results.hidden = true;
			results.innerHTML = "";
		}

		function score(item, q) {
			var t = (item.title || "").toLowerCase();
			var d = (item.desc || "").toLowerCase();
			if (t === q) return 100;
			if (t.indexOf(q) === 0) return 80;
			if (t.indexOf(q) > -1) return 60;
			if (d.indexOf(q) > -1) return 30;
			return 0;
		}

		input.addEventListener("input", function () {
			var q = input.value.trim().toLowerCase();
			if (q.length < 2) {
				hide();
				return;
			}

			var matches = data
				.map(function (item) {
					return { item: item, score: score(item, q) };
				})
				.filter(function (x) {
					return x.score > 0;
				})
				.sort(function (a, b) {
					return b.score - a.score;
				})
				.slice(0, 8);

			if (!matches.length) {
				results.hidden = false;
				results.innerHTML = "";
				var emptyDiv = document.createElement("div");
				emptyDiv.className = "search-empty";
				emptyDiv.textContent = texts.no_results || "No results found.";
				results.appendChild(emptyDiv);
				return;
			}

			results.innerHTML = "";
			matches.forEach(function (m) {
				var item = m.item;
				var desc = item.desc || "";

				var a = document.createElement("a");
				a.className = "search-item";
				a.href = isSafeRelativeUrl(item.url) ? item.url : "#";

				var titleDiv = document.createElement("div");
				titleDiv.className = "search-item-title";
				titleDiv.textContent = item.title;

				var descDiv = document.createElement("div");
				descDiv.className = "search-item-desc";
				descDiv.textContent = desc;

				a.appendChild(titleDiv);
				a.appendChild(descDiv);
				results.appendChild(a);
			});
			results.hidden = false;
		});

		document.addEventListener("click", function (e) {
			if (!results.contains(e.target) && e.target !== input) {
				hide();
			}
		});

		input.addEventListener("keydown", function (e) {
			if (e.key === "Escape") {
				hide();
				input.blur();
			}
		});
	}

	function initQRCodes() {
		var qrContainers = document.querySelectorAll(".qr-code");
		qrContainers.forEach(function (container) {
			var text = container.getAttribute("data-qr-text");
			if (!text || typeof QRCode === "undefined") return;
			container.innerHTML = "";
			try {
				new QRCode(container, {
					text: text,
					width: 128,
					height: 128,
					colorDark: "#000000",
					colorLight: "#ffffff",
					correctLevel: QRCode.CorrectLevel.M,
				});
			} catch (e) {
				container.innerHTML = "QR error";
			}
		});
	}

	function initCopyFromDataButtons(texts) {
		var copyButtons = document.querySelectorAll(".btn-copy");
		copyButtons.forEach(function (btn) {
			btn.addEventListener("click", function () {
				var text = btn.getAttribute("data-clipboard-text");
				if (!text) return;
				navigator.clipboard.writeText(text).then(function () {
					var originalText = btn.innerText;
					btn.innerText = texts.copied || "Copied";
					btn.classList.add("copied");
					setTimeout(function () {
						btn.innerText = originalText;
						btn.classList.remove("copied");
					}, 1200);
				});
			});
		});
	}

	function initScrollReveal() {
		if (!("IntersectionObserver" in window)) {
			// Fallback: make all elements visible immediately.
			document.querySelectorAll(".fade-in").forEach(function (el) {
				el.classList.add("is-visible");
			});
			return;
		}
		var observer = new IntersectionObserver(
			function (entries) {
				entries.forEach(function (entry) {
					if (entry.isIntersecting) {
						entry.target.classList.add("is-visible");
						observer.unobserve(entry.target);
					}
				});
			},
			{ threshold: 0.12 }
		);
		document.querySelectorAll(".fade-in").forEach(function (el) {
			observer.observe(el);
		});
	}

	function initThemeSwitcher() {
		var dropdown = document.querySelector(".theme-dropdown");
		var trigger = document.querySelector(".theme-trigger");
		var options = document.querySelectorAll(".theme-option");
		if (!dropdown || !trigger) return;

		function applyTheme(theme) {
			if (theme === "dark" || theme === "light") {
				document.documentElement.setAttribute("data-theme", theme);
			} else {
				theme = "auto";
				document.documentElement.removeAttribute("data-theme");
			}
			try {
				localStorage.setItem("pgarachne_theme", theme);
			} catch (e) {}
			options.forEach(function (opt) {
				opt.classList.toggle("active", opt.getAttribute("data-theme-value") === theme);
			});
		}

		var stored;
		try {
			stored = localStorage.getItem("pgarachne_theme");
		} catch (e) {}
		applyTheme(stored || "auto");

		options.forEach(function (opt) {
			opt.addEventListener("click", function () {
				applyTheme(opt.getAttribute("data-theme-value"));
				dropdown.classList.remove("open");
				trigger.setAttribute("aria-expanded", "false");
			});
		});
	}

	function initDropdownToggles() {
		var dropdowns = document.querySelectorAll(".theme-dropdown, .lang-dropdown");

		dropdowns.forEach(function (dropdown) {
			var trigger = dropdown.querySelector(".theme-trigger, .lang-trigger");
			if (!trigger) return;
			trigger.addEventListener("click", function (e) {
				e.stopPropagation();
				var wasOpen = dropdown.classList.contains("open");
				dropdowns.forEach(function (d) {
					d.classList.remove("open");
					var t = d.querySelector(".theme-trigger, .lang-trigger");
					if (t) t.setAttribute("aria-expanded", "false");
				});
				if (!wasOpen) {
					dropdown.classList.add("open");
					trigger.setAttribute("aria-expanded", "true");
				}
			});
		});

		document.addEventListener("click", function () {
			dropdowns.forEach(function (d) {
				d.classList.remove("open");
				var t = d.querySelector(".theme-trigger, .lang-trigger");
				if (t) t.setAttribute("aria-expanded", "false");
			});
		});

		document.addEventListener("keydown", function (e) {
			if (e.key === "Escape") {
				dropdowns.forEach(function (d) {
					d.classList.remove("open");
					var t = d.querySelector(".theme-trigger, .lang-trigger");
					if (t) t.setAttribute("aria-expanded", "false");
				});
			}
		});
	}

	document.addEventListener("DOMContentLoaded", function () {
		var texts = parseJSONScript("site-search-texts", {});
		var searchData = parseJSONScript("site-search-data", []);

		initQRCodes();
		initCopyFromDataButtons(texts);
		addCopyButtons(texts);
		initSearch(searchData, texts);
		initScrollReveal();
		initThemeSwitcher();
		initDropdownToggles();
	});
})();
