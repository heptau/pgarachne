---
title: "SSE Tester"
description: "PgArachne SSE Tester - Τεκμηρίωση"
---

<section id="sse-tester">
<h2>SSE Tester</h2>
<p>Το <strong>SSE Tester</strong> είναι ένα εργαλείο browser σε ένα αρχείο, που βρίσκεται στο <code>tools/test-sse</code>. Σας επιτρέπει να εγγραφείτε σε οποιονδήποτε αριθμό καναλιών <code>NOTIFY</code> της PostgreSQL μέσω μιας ζωντανής σύνδεσης <a href="../../real-time-notifications/">Server-Sent Events</a> και να παρακολουθείτε τα εισερχόμενα συμβάντα σε πραγματικό χρόνο.</p>

<h3>Λειτουργίες</h3>
<ul>
<li><strong>Εγγραφές πολλαπλών καναλιών</strong> — προσθέστε ή αφαιρέστε κανάλια ως ετικέτες πριν τη σύνδεση. Όλα τα κανάλια περνιούνται ως παράμετρος ερωτήματος <code>?channels=</code> διαχωρισμένη με κόμμα.</li>
<li><strong>Όλες οι μέθοδοι πιστοποίησης</strong> — η καρτέλα <em>Password</em> χρησιμοποιεί HTTP Basic Auth (άμεσα διαπιστευτήρια, χωρίς ανάγκη JWT)· η καρτέλα <em>Bearer Token</em> δέχεται ένα JWT ή API token.</li>
<li><strong>Ζωντανή ροή</strong> — χρησιμοποιεί το Fetch Streams API με έναν <code>AbortController</code> για καθαρή αποσύνδεση. Οι γραμμές SSE αναλύονται χειροκίνητα ώστε τα πεδία <code>event:</code>, <code>data:</code>, και heartbeat (<code>:</code>) να αντιμετωπίζονται σωστά.</li>
<li><strong>Επισήμανση σύνταξης JSON</strong> — τα payloads δεδομένων που είναι έγκυρο JSON εμφανίζονται μορφοποιημένα με έγχρωμη κωδικοποίηση.</li>
<li><strong>Διατηρούμενες ρυθμίσεις</strong> — το API URL, το prefix, η βάση δεδομένων, τα στοιχεία σύνδεσης, και η λίστα καναλιών αποθηκεύονται στο <code>localStorage</code>.</li>
</ul>

<h3>Πιστοποίηση</h3>
<p>Το εργαλείο επαληθεύει τα διαπιστευτήρια πριν αποκαλύψει τον πίνακα εγγραφής. Κάνοντας κλικ στο <strong>Verify &amp; Continue</strong> καλείται η <code>capabilities</code> με την επιλεγμένη κεφαλίδα — εάν το PgArachne επιστρέψει επιτυχή απόκριση, ο πίνακας εγγραφής ξεκλειδώνεται.</p>

<ul>
<li><strong>Καρτέλα Password</strong> — στέλνει <code>Authorization: Basic &lt;base64(user:pass)&gt;</code>. Ο χρήστης της βάσης δεδομένων πρέπει να έχει <code>GRANT EXECUTE</code> στις συναρτήσεις-στόχους και πρόσβαση στο endpoint SSE· δεν απαιτείται <code>GRANT &lt;role&gt; TO pgarachne</code>.</li>
<li><strong>Καρτέλα Bearer Token</strong> — στέλνει <code>Authorization: Bearer &lt;token&gt;</code>. Επικολλήστε ένα JWT (που αποκτήθηκε μέσω του <a href="../get-jwt/">JWT Getter</a>) ή ένα μακρόβιο API token.</li>
</ul>

<div class="tip">
<strong>Πώς να το ενεργοποιήσετε:</strong> Ρυθμίστε το <code>STATIC_FILES_PATH</code> σε <code>tools/test-sse</code> και επισκεφθείτε το <code>http://localhost:8080</code>. Εναλλακτικά, ανοίξτε απευθείας το <code>index.html</code> σε έναν browser — όλες οι λειτουργίες δουλεύουν χωρίς τοπικό server.
</div>
</section>
