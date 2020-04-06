'use strict';

(function() {

const message = document.getElementById('message');
const send = document.getElementById('send');
const scan = document.getElementById('scan');
const verify = document.getElementById('verify');
const list_verifications = document.getElementById('list_verifications');
const infection = document.getElementById('infection');
const output = document.getElementById('output');

send.onclick = function() {
  const SEPARATOR = '\n\n-----------------------------------\n';
  var request = new XMLHttpRequest();
  const URL = 'https://us-central1-apollo-server-273118.cloudfunctions.net/prototype1';
  request.open('POST', URL);
  request.setRequestHeader('Content-Type', 'application/json');
  var messageFixed = message.value.replace(/[/][/][^\n]*/g, '\n');
  try {
    var messagePacked = JSON.stringify(JSON.parse(messageFixed));
    request.send(messagePacked);
    request.onreadystatechange = function() {
      if (this.readyState === XMLHttpRequest.DONE) {
        if (this.status === 200) {
          output.innerText = request.responseText + SEPARATOR + output.innerText;
        } else {
          output.innerText = 'ERROR: HTTP/' + this.status +
                             '\n' + request.responseText + 
                             SEPARATOR + output.innerText;
        }
      }
    };
  } catch(e) {
    output.innerText = 'ERROR: JSON DOESNT PARSE' + SEPARATOR + output.innerText;
  }
};

infection.onclick = function() {
  message.value = `
{
  "method": "infection",
  "photo": "base64 encoded image of some sort of proof",
  "circumstance": "self-report", // self-report / close-contact / test-result
  "verifier": "Name of the verifier",
  "ids": [
    // List of infected ids.
    "0123456789ABCDEF"
    // ...
  ]
}
  `;
};

scan.onclick = function() {
  message.value = `
{
  "method": "scan",
  "prefixes": [
    // List of id prefixes.
    "0123"
    // ...
  ]
  // ,"cursor": "optional prior server cursor"
}
  `;
};

list_verifications.onclick = function() {
  message.value = `
{
  "method": "list_verifications",
  "verifier": "Name of the verifier"
}
  `;
};

verify.onclick = function() {
  message.value = `
{
  "method": "verify",
  "verifier": "Name of the verifier",
  "batch_id": "database key of previously sent id batch"
}
  `;
};

scan.onclick();

})();
