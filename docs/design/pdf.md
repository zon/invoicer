# PDF Design

Every stored invoice has a PDF form. The web app renders it on request from stored data. The PDF is never stored.

## Invoice PDF

The PDF is a printable form of one [invoice](invoices.md). It shows the same data as the invoice detail: vendor, customer, project when set, month and year, invoice identifier, hourly rate, every weekly line item, and total. It shows neither the paid status nor a paid marker. Each weekly line item shows its date range and hours. The total comes from stored data, as [Invoices](invoices.md) describes.

The PDF is one or more A4 portrait pages. It contains no form, no links, and no other invoice.

## Download

The download address is the detail address plus `/pdf`, like `/2027/oct/pdf`. The response is a PDF file named like `invoice-2027-10.pdf`, so the browser downloads it rather than opening it as a page. The month in the filename uses two digits.

A download changes nothing. Every download renders the latest stored values for the invoice.

## Access

Every download needs a valid session, like every page. Any signed-in user downloads any invoice they can read, as [Access](access.md) describes. The paid status does not restrict download.

A request without a session goes to sign-in.

## Rendering

The app renders the PDF inside the same process that serves the web app. It uses a Go PDF library, as [Stack](../stack.md) describes. Rendering needs no browser, no external tool, and no support file beside the binary. The font is embedded in the binary.

Stored values are text. A customer name with punctuation or non-Latin letters renders as text and never as markup.

A failed render responds with an error and sends no partial document.

## Missing Invoice

An unknown year, an unknown month, or a month with no stored invoice returns 404, like the detail page.
