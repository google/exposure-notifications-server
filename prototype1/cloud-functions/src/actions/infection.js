'use strict';

const storage = require('../storage');

/**
 * Handle an infection declaration request.
 *
 * @param {Object} request JSON infection request.
 * @param {!express:Response} response HTTP response context.
 */
exports.handle = (request, response) => {
  if (request.ids === undefined) {
    response.status(400).send('Bad Request - missing ids');
    return;
  }
  if (request.ids.length === 0) {
    response.status(400).send('Bad Request - ids list empty');
    return;
  }
  const batch_id = storage.uuid();
  let rows = [];
  for (var i = 0; i < request.ids.length; i++) {
    if ((typeof request.ids[i]) !== 'string' ||
        !request.ids[i].match(storage.ID_REGEX)) {
      response.status(400).send(
          'Bad Request - invalid id at ' + i + ': "' +
          request.ids[i].toString() + '"');
      return;
    }
    rows.push({
      key: request.ids[i],
      data: {
        properties: {
          batch_id: batch_id,
        },
      },
    });
  }
  Promise.resolve().then(() => {
    return storage.verifications.insert({
      key: batch_id,
      data: {
        properties: {
          circumstance: request.circumstance,
          photo: request.photo,
          verifier: request.verifier,
        },
      },
    });
  }).then(() => {
    return storage.infections.insert(rows);
  }).then(() => {
    response.status(200).send({
      'status': 'accepted',
    });
  });
};
