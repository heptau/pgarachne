---
title: "JWT Signer"
description: "PgArachne JWT Signer"
---

<section id="jwt-signer">
<h2>JWT Signer</h2>
<p>Το <strong>JWT Signer</strong> είναι ένα εργαλείο προγράμματος περιήγησης σε ένα αρχείο, στο <code>tools/jwt-signer</code>. Σε αντίθεση με το <a href="../get-jwt/">JWT Getter</a>, δεν επικοινωνεί ποτέ με το PgArachne ή τη βάση δεδομένων: εισάγετε χειροκίνητα το <code>JWT_SECRET</code> και το token υπογράφεται (HS256) απευθείας στο πρόγραμμα περιήγησής σας.</p>
<p>Διαδικτυακή έκδοση: <a href="https://jwt-signer.pgarachne.com" target="_blank" rel="noopener">jwt-signer.pgarachne.com</a></p>

<figure class="tool-screenshot">
<img src="/assets/pgarachne-tool-jwt-signer.webp" alt="JWT Signer" loading="lazy">
</figure>

<div class="warning">
<strong>Χειριστείτε το μυστικό JWT με προσοχή</strong>
<ul>
<li>Το <code>JWT_SECRET</code> είναι το κύριο κλειδί του API σας: όποιος το γνωρίζει μπορεί να υπογράψει token για <em>οποιονδήποτε</em> ρόλο βάσης δεδομένων, ακόμη και για superuser.</li>
<li><strong>Ποτέ μην επικολλάτε μυστικό παραγωγής σε ιστοσελίδα που δεν ελέγχετε πλήρως.</strong> Χρησιμοποιήστε τη φιλοξενούμενη έκδοση μόνο με μυστικά ανάπτυξης/δοκιμών. Στην παραγωγή αποθηκεύστε το <code>index.html</code> και ανοίξτε το τοπικά (λειτουργεί εκτός σύνδεσης) ή υπογράψτε τα token στον διακομιστή.</li>
<li>Όλα εκτελούνται στο πρόγραμμα περιήγησής σας. Η σελίδα απαγορεύει κάθε πρόσβαση στο δίκτυο (Content Security Policy <code>connect-src 'none'</code>), δεν φορτώνει εξωτερικούς πόρους και <strong>δεν αποθηκεύει ποτέ το μυστικό</strong> — ούτε στο <code>localStorage</code>, ούτε σε cookies, ούτε στο URL. Θυμάται μόνο ρόλο, βάση, διάρκεια, issuer και audience.</li>
<li>Ένα token που υπογράφεται εδώ γίνεται δεκτό από το PgArachne ακριβώς όπως ένα token από το <code>/token</code>. Επιλέξτε μικρή διάρκεια και μην μοιράζεστε ποτέ token υπογεγραμμένα με πραγματικό μυστικό.</li>
</ul>
</div>

<h3>Πότε να το χρησιμοποιήσετε</h3>
<ul>
<li>Ανάπτυξη και δοκιμές όταν δεν θέλετε (ή δεν μπορείτε) να συνδεθείτε με κωδικό της βάσης δεδομένων.</li>
<li>Έλεγχος του πώς χειρίζεται ο πελάτης σας τη λήξη: αρνητική διάρκεια παράγει ήδη ληγμένο token.</li>
<li>Δοκιμή <code>iss</code> / <code>aud</code> και πρόσθετων claims σε διακομιστή που τα απαιτεί.</li>
<li>Δημιουργία τυχαίου μυστικού 256 bit για νέο <code>JWT_SECRET</code> με ένα κλικ.</li>
</ul>

<div class="tip">
<strong>Πώς να το ενεργοποιήσετε:</strong> Ορίστε το <code>STATIC_FILES_PATH</code> σε <code>tools/jwt-signer</code> και επισκεφθείτε το <code>http://localhost:8080</code>, ή ανοίξτε το <code>index.html</code> απευθείας στο πρόγραμμα περιήγησης — λειτουργεί χωρίς διακομιστή.
</div>
</section>
