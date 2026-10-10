---
title: "Εργαλεία"
description: "Εργαλεία για το PgArachne - Τεκμηρίωση."
menu:
  main:
    name: "Εργαλεία"
    weight: 80
---

<section id="tools">
<h2>Εργαλεία</h2>
<p>Το PgArachne υποστηρίζεται από ένα σύνολο εργαλείων βασισμένων σε browser που απλοποιούν την ανάπτυξη, τη δοκιμή, και την εξερεύνηση του API. Όλα τα εργαλεία είναι αυτόνομα αρχεία HTML — χωρίς βήμα build, χωρίς εξαρτήσεις.</p>

<div class="tools-grid">
<div class="card">
<h3>PgArachne Explorer</h3>
<p>Ένα πλήρως λειτουργικό web UI για την περιήγηση στο API σας, τη δοκιμή συναρτήσεων, και την προβολή αυτόματα παραγόμενης τεκμηρίωσης. Υποστηρίζει τόσο άμεσα διαπιστευτήρια (HTTP Basic Auth) όσο και πιστοποίηση με Bearer token.</p>
<p><a href="api-explorer/" class="btn stretched-link">Μάθετε περισσότερα για το Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Εγγραφείτε σε ένα ή περισσότερα κανάλια NOTIFY της PostgreSQL μέσω μιας ζωντανής σύνδεσης Server-Sent Events. Υποστηρίζει όλες τις τρεις μεθόδους πιστοποίησης και εμφανίζει συμβάντα JSON με επισήμανση σύνταξης.</p>
<p><a href="sse-tester/" class="btn stretched-link">Μάθετε περισσότερα για το SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Ανταλλάξτε ένα όνομα χρήστη και κωδικό πρόσβασης PostgreSQL για ένα βραχύβιο JWT μέσω του endpoint <code>/token</code> (διαπιστευτήρια HTTP Basic). Εμφανίζει το αποκωδικοποιημένο payload και τον χρόνο λήξης — χρήσιμο για την αποσφαλμάτωση ροών βασισμένων σε tokens.</p>
<p><a href="get-jwt/" class="btn stretched-link">Μάθετε περισσότερα για το JWT Getter</a></p>
</div>

<div class="card">
<h3>JWT Signer</h3>
<p>Δημιουργήστε JWT τοπικά στο πρόγραμμα περιήγησης, εισάγοντας χειροκίνητα το <code>JWT_SECRET</code> — χωρίς βάση δεδομένων ή διακομιστή. Ορίστε ρόλο, βάση, διάρκεια και πρόσθετα claims, ακόμη και ήδη ληγμένο token για δοκιμές. Με όλες τις προειδοποιήσεις ασφαλείας.</p>
<p><a href="jwt-signer/" class="btn stretched-link">Περισσότερα για το JWT Signer</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Μια εγγενής εφαρμογή macOS που βρίσκεται στη γραμμή μενού σας. Διαχειριστείτε πολλαπλές εγκαταστάσεις PgArachne, δείτε ζωντανά logs, και παρακολουθήστε μετρήσεις με ένα μόνο κλικ.</p>
<p><a href="macos-toolbar/" class="btn stretched-link">Εξερευνήστε τις Λειτουργίες του Toolbar</a></p>
</div>
</div>
</section>
