'use strict';

/**
 * Handle an infection declaration request.
 *
 * @param {Object} request JSON infection request.
 * @param {!express:Response} response HTTP response context.
 */
exports.handle = (request, response) => {
  response.status(200).send(JSON.stringify(request));
};
