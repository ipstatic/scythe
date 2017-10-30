Scythe
======

Scythe is a [Go](https://golang.org) program that translates [Harvest](https://www.getharvest.com)
time entries into a specific format in a [Google Spreadsheet](https://docs.google.com/spreadsheets).

## Setup

First you will need to get credentials from the [Google API](https://console.developers.google.com/apis)
so that you can communicate with a spreadsheet. Follow this [guide](https://www.twilio.com/blog/2017/02/an-easy-way-to-read-and-write-to-a-google-spreadsheet-in-python.html)
to help you setup a service account. In the end you will have a client_secret.json
file in this repository. Be sure you have both the Drive API and Sheets API assigned
to the service account you create.

Next you will need to add the email address of the service account to the spreadsheet
you are going to edit. The email address is inside the client_secret.json file
described above. Just click share, advanced, put the email address in the
Invite People field, uncheck Notify people, and click OK.

Finally you will need to fill out the required Harvest details in scythe.yml.
