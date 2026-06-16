# Fonts

`DejaVuSansCondensed.ttf` and `DejaVuSansCondensed-Bold.ttf` are embedded into
the NIS2 compliance PDF (`controllers/compliance_pdf.go`) so Polish diacritics
(ł ą ś ż ń ę ó ć ź and uppercase) render correctly — fpdf's built-in core fonts
are Latin-1 only.

**DejaVu Fonts** — released under a permissive, redistributable license
(Bitstream Vera Fonts License + Arev Fonts License). Embedding in documents and
redistribution are explicitly permitted. Source and full license:
https://dejavu-fonts.github.io/License.html

Copies bundled here originate from the `github.com/go-pdf/fpdf` module's `font/`
directory.
