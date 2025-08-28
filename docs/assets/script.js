document.addEventListener("DOMContentLoaded", function () {
	// --- QR Code Generation ---
	const qrContainers = document.querySelectorAll('.qr-code');

	qrContainers.forEach(container => {
		const text = container.getAttribute('data-qr-text');
		if (text) {
			// Clear placeholder text
			container.innerHTML = "";

			try {
				new QRCode(container, {
					text: text,
					width: 128,
					height: 128,
					colorDark: "#000000",
					colorLight: "#ffffff",
					correctLevel: QRCode.CorrectLevel.M
				});
			} catch (e) {
				console.error("QR Code Error:", e);
				container.innerHTML = "Error loading QR";
			}
		}
	});

	// --- Copy to Clipboard ---
	const copyButtons = document.querySelectorAll('.btn-copy');

	copyButtons.forEach(btn => {
		btn.addEventListener('click', function () {
			const textToCopy = this.getAttribute('data-clipboard-text');

			if (textToCopy) {
				navigator.clipboard.writeText(textToCopy).then(() => {
					// Feedback
					const originalText = this.innerText;
					this.innerText = "Copied!";
					this.classList.add('copied');

					setTimeout(() => {
						this.innerText = originalText;
						this.classList.remove('copied');
					}, 2000);
				}).catch(err => {
					console.error('Failed to copy: ', err);
				});
			}
		});
	});
});
