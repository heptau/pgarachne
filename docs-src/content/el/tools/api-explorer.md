---
title: "PgArachne Explorer"
description: "PgArachne Explorer - PgArachne"
---

<section id="explorer">
<h2>PgArachne Explorer</h2>
<p>Το <strong>Explorer</strong> είναι ένα ισχυρό web GUI που περιλαμβάνεται στον κατάλογο <code>tools/pgarachne-explorer</code>. Δεν είναι
		απλά ένα εργαλείο τεκμηρίωσης — είναι μια πλήρως λειτουργική <strong>εφαρμογή επίδειξης</strong> χτισμένη με HTML/JS
		που επικοινωνεί με τη βάση δεδομένων αποκλειστικά μέσω του PgArachne. Μια φιλοξενούμενη έκδοση είναι επίσης διαθέσιμη στο
<a href="https://explorer.pgarachne.com" target="_blank">explorer.pgarachne.com</a>.</p>

<p><strong>Τι μπορεί να κάνει;</strong></p>
<ul>
<li><strong>Επιθεώρηση API:</strong> Διαβάζει τη συνάρτηση <code>capabilities</code> για να εμφανίσει όλα τα διαθέσιμα
			endpoints και τις παραμέτρους τους.</li>
<li><strong>Ζωντανή Δοκιμή:</strong> Μπορείτε να εκτελέσετε συναρτήσεις απευθείας από τον browser.</li>
<li><strong>Αυτόματη Τεκμηρίωση:</strong> Αποδίδει τα σχόλια SQL (συμπεριλαμβανομένων των μεταδεδομένων
			<code>--- PARAMS ---</code>) σε ευανάγνωστη τεκμηρίωση.</li>
<li><strong>Μοντέρνο UI:</strong> Διαθέτει υποστήριξη λειτουργίας Dark/Light και μπορεί να εγκατασταθεί ως <strong>PWA</strong> σε κινητές συσκευές.</li>
<li><strong>Πιστοποίηση:</strong> Η καρτέλα <em>Password</em> χρησιμοποιεί HTTP Basic Auth για απευθείας σύνδεση ως ο χρήστης της βάσης δεδομένων — δεν απαιτείται <code>GRANT &hellip; TO pgarachne</code>. Η καρτέλα <em>API Token</em> δέχεται ένα JWT ή μακρόβιο API token που αποστέλλεται ως <code>Bearer</code>.</li>
</ul>

<p><strong>Παράμετροι URL:</strong></p>
<p>Μπορείτε να προσυμπληρώσετε τις ρυθμίσεις σύνδεσης χρησιμοποιώντας την παράμετρο <code>?url=</code>, για παράδειγμα: 
<code>?url=http://localhost:8080/db/my_database</code>. Αυτό θα χωρίσει αυτόματα το URL στα πεδία API Host, Prefix, και Database.</p>

<div class="tip">
<strong>Πώς να το ενεργοποιήσετε:</strong> Ρυθμίστε τη μεταβλητή περιβάλλοντος <code>STATIC_FILES_PATH</code> ώστε να δείχνει στον
φάκελο <code>tools/pgarachne-explorer</code> στον δίσκο σας. Στη συνέχεια, επισκεφθείτε το <code>http://localhost:8080</code>.
</div>
</section>
