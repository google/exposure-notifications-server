'use strict';

const storage = require('../storage');

const MAX_RESULTS = 400;

/**
 * Handle a scan request.
 *
 * @param {Object} request JSON infection request.
 * @param {!express:Response} response HTTP response context.
 */
exports.handle = (request, response) => {
  // Reject incorrectly formatted requests.
  if (request.prefixes === undefined) {
    response.status(400).send('Bad Request - missing prefixes');
    return;
  }
  if (request.prefixes.length === 0) {
    response.status(400).send('Bad Request - prefixes list empty');
    return;
  }
  if (request.cursor !== undefined &&
      (typeof request.cursor) !== 'string') {
    response.status(400).send(
        'Bad Request - invalid cursor "' + request.cursor + '"');
    return;
  }

  // Gather and sort prefixes.
  var prefixes = [];
  for (var i = 0; i < request.prefixes.length; i++) {
    if ((typeof request.prefixes[i]) !== 'string' ||
        !request.prefixes[i].match(storage.PREFIX_REGEX)) {
      response.status(400).send(
          'Bad Request - invalid prefix at ' + i + ': "' +
          request.prefixes[i].toString() + '"');
      return;
    }
    prefixes.push(request.prefixes[i]);
  }
  prefixes.sort();

  // Transform into clipped ranges based on the cursor.
  const cursor = request.cursor || '';
  let ranges = [];
  for (var i = 0; i < request.prefixes.length; i++) {
    let start = request.prefixes[i];
    let end = request.prefixes[i] + '\xff';
    if (start >= cursor) {
      ranges.push({ start: start, end: end });
    } else if (end > cursor) {
      ranges.push({ start: cursor, end: end });
    }
  }

  // Issue query and return result.
  var results = [];
  storage.infections.createReadStream({
    ranges: ranges,
    limit: MAX_RESULTS,
  }).on('error', err => {
    response.status(500).send(
        'Bad Request - prefix read failure, size ' + results.length);
  }).on('data', function(data) {
    results.push(data.id);
  }).on('end', function() {
    var reply = {
      'status': 'accepted',
      'ids': results,
    }
    if (results.length === MAX_RESULTS) {
      reply.cursor = results[results.length - 1] + '\x01';
    } 
    response.status(200).send(JSON.stringify(reply));
  });
};
