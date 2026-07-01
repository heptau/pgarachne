---
title: "JWT Getter"
description: "PgArachne JWT Getter - Τεκμηρίωση"
---

<section id="get-jwt">
<h2>JWT Getter</h2>
<p>Το <strong>JWT Getter</strong> είναι ένα ελάχιστο εργαλείο browser σε ένα αρχείο, που βρίσκεται στο <code>tools/get-jwt</code>. Ανταλλάζει ένα όνομα χρήστη και κωδικό πρόσβασης PostgreSQL για ένα βραχύβιο JWT καλώντας τη μέθοδο JSON-RPC <code>get_jwt</code> — και σας δείχνει το αποκωδικοποιημένο payload και τον χρόνο λήξης.</p>

<h3>Πότε να το χρησιμοποιήσετε</h3>
<p>Χρησιμοποιήστε το JWT Getter όταν χρειάζεστε ένα token για έναν από τους ακόλουθους σκοπούς:</p>
<ul>
<li>Επικόλληση ενός token στο <a href="../api-explorer/">Explorer</a> ή στο <a href="../sse-tester/">SSE Tester</a> (καρτέλα Bearer Token).</li>
<li>Δοκιμή της λήξης JWT ή των claims <code>db_role</code> / <code>db_name</code> στην εφαρμογή σας.</li>
<li>Ταχεία επιβεβαίωση ότι ο χρήστης συστήματος <code>pgarachne</code> μπορεί να εναλλάξει σε συγκεκριμένο ρόλο (το <code>GRANT &lt;role&gt; TO pgarachne</code> πρέπει να υπάρχει).</li>
</ul>

<div class="tip">
<strong>Σημείωση:</strong> Το <code>get_jwt</code> απαιτεί <code>GRANT &lt;role&gt; TO pgarachne</code> στη βάση δεδομένων επειδή το PgArachne επαληθεύει τον κωδικό πρόσβασης εσωτερικά μέσω <code>SET LOCAL ROLE</code>. Εάν θέλετε να παραλείψετε αυτό το grant, χρησιμοποιήστε άμεσα διαπιστευτήρια (HTTP Basic Auth) στο <a href="../api-explorer/">Explorer</a> ή στο <a href="../sse-tester/">SSE Tester</a> αντ' αυτού.
</div>

<h3>Λειτουργίες</h3>
<ul>
<li>Εισάγετε το API URL, το prefix, τη βάση δεδομένων, το όνομα χρήστη και τον κωδικό πρόσβασης — πατήστε <kbd>Enter</kbd> ή κάντε κλικ στο <strong>Get JWT</strong>.</li>
<li>Το ανεπεξέργαστο token εμφανίζεται και μπορεί να αντιγραφεί με ένα κλικ.</li>
<li>Το payload του JWT αποκωδικοποιείται με Base64 και εμφανίζεται με επισήμανση σύνταξης JSON.</li>
<li>Ένα badge λήξης εμφανίζει τον υπόλοιπο χρόνο ζωής (ή επισημαίνει το token ως ληγμένο).</li>
<li>Οι ρυθμίσεις σύνδεσης και η σύνδεση αποθηκεύονται στο <code>localStorage</code>.</li>
</ul>

<h3>Πώς να το ενεργοποιήσετε</h3>
<p>Ρυθμίστε το <code>STATIC_FILES_PATH</code> σε <code>tools/get-jwt</code> και επισκεφθείτε το <code>http://localhost:8080</code>, ή ανοίξτε απευθείας το <code>index.html</code> σε έναν browser.</p>
</section>
