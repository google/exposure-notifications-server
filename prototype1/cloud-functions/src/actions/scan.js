'use strict';

/**
 * Handle a scan request.
 *
 * @param {Object} request JSON infection request.
 * @param {!express:Response} response HTTP response context.
 */
exports.handle = (request, response) => {
  response.status(501).send('Not implemented');
};
