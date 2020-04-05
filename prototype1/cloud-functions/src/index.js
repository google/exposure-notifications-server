'use strict';

const infection = require('./actions/infection.js');
const list_verifications = require('./actions/list_verifications.js');
const scan = require('./actions/scan.js');
const verify = require('./actions/verify.js');

/**
 * Responds to any HTTP request.
 *
 * @param {!express:Request} request HTTP request context.
 * @param {!express:Response} response HTTP response context.
 */
exports.api = (request, response) => {
  // Allow non-same-origin requests.
  // TODO: Tighten this down.
  response.set('Access-Control-Allow-Origin', '*');
  if (request.method === 'OPTIONS') {
    // Send response to OPTIONS requests
    response.set('Access-Control-Allow-Methods', '*');
    response.set('Access-Control-Allow-Headers', 'Content-Type');
    response.set('Access-Control-Max-Age', '3600');
    response.status(204).send('');
    return;
  }

  // Require JSON POST requests.
  if (request.method !== 'POST') {
    response.status(400).send('Bad Request - must be POST');
  }
  if (request.get('Content-Type') !== 'application/json') {
    response.status(400).send('Bad Request - Content-Type: application/json');
    return;
  }

  const DISPATCH = {
    infection: infection.handle,
    list_verifications: list_verifications.handle,
    scan: scan.handle,
    verify: verify.handle,
  };
  const message = request.body;
  const handler = DISPATCH[message.method];
  if (handler === undefined) {
    response.status(400).send(
        'Bad Request - unsupported method "' + message['method'] + '"');
    return;
  }
  handler(request, response);
};
