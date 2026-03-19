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
			var parsed = JSON.parse(el.textContent);
			if (typeof parsed === "string") {
				var trimmed = parsed.trim();
				if ((trimmed[0] === "{" && trimmed[trimmed.length - 1] === "}") || (trimmed[0] === "[" && trimmed[trimmed.length - 1] === "]")) {
					return JSON.parse(parsed);
				}
			}
			return parsed;
		} catch (err) {
			return fallback;
		}
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
				a.href = item.url;

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

	document.addEventListener("DOMContentLoaded", function () {
		var texts = parseJSONScript("site-search-texts", {});
		var searchData = parseJSONScript("site-search-data", []);

		initQRCodes();
		initCopyFromDataButtons(texts);
		addCopyButtons(texts);
		initSearch(searchData, texts);
	});
})();
