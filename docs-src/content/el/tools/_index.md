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
<p><a href="api-explorer/" class="btn">Μάθετε περισσότερα για το Explorer</a></p>
</div>

<div class="card">
<h3>SSE Tester</h3>
<p>Εγγραφείτε σε ένα ή περισσότερα κανάλια NOTIFY της PostgreSQL μέσω μιας ζωντανής σύνδεσης Server-Sent Events. Υποστηρίζει όλες τις τρεις μεθόδους πιστοποίησης και εμφανίζει συμβάντα JSON με επισήμανση σύνταξης.</p>
<p><a href="sse-tester/" class="btn">Μάθετε περισσότερα για το SSE Tester</a></p>
</div>

<div class="card">
<h3>JWT Getter</h3>
<p>Ανταλλάξτε ένα όνομα χρήστη και κωδικό πρόσβασης PostgreSQL για ένα βραχύβιο JWT μέσω της μεθόδου <code>get_jwt</code>. Εμφανίζει το αποκωδικοποιημένο payload και τον χρόνο λήξης — χρήσιμο για την αποσφαλμάτωση ροών βασισμένων σε tokens.</p>
<p><a href="get-jwt/" class="btn">Μάθετε περισσότερα για το JWT Getter</a></p>
</div>

<div class="card">
<h3>PgArachne Toolbar (macOS)</h3>
<p>Μια εγγενής εφαρμογή macOS που βρίσκεται στη γραμμή μενού σας. Διαχειριστείτε πολλαπλές εγκαταστάσεις PgArachne, δείτε ζωντανά logs, και παρακολουθήστε μετρήσεις με ένα μόνο κλικ.</p>
<p><a href="macos-toolbar/" class="btn">Εξερευνήστε τις Λειτουργίες του Toolbar</a></p>
</div>
</div>
</section>
