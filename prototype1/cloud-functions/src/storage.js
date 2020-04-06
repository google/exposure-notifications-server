'use strict';

const bigtable = require('@google-cloud/bigtable');
const uuid = require('uuid');

const bigtableClient = bigtable();
const instance = bigtableClient.instance('prototype1');
const infections = instance.table('infections');
const verifications = instance.table('verifications');
const verifiers = instance.table('verifiers');
const prefix_index = instance.table('prefix_index');

exports.uuid = function() {
  return uuid.v5('apollo-prototype1', uuid.v5.URL);
};

exports.infections = infections;
exports.verifications = verifications;
exports.verifiers = verifiers;
exports.prefix_index = prefix_index;
exports.ID_REGEX = /^[A-Za-z0-9/+]{16,32}/;
exports.PREFIX_REGEX = /^[A-Za-z0-9/+]{4,32}/;
