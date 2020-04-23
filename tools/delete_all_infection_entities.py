# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
This script will delete all entitities of the infection kind
"""
from google.cloud import datastore
import itertools
from concurrent.futures import ThreadPoolExecutor


from queue import Queue
from threading import Thread

client = datastore.Client()


def producer(futures_queue):
    query = client.query(kind="infection")
    all_infection_entity_keys = (entity.key for entity in query.fetch())

    with ThreadPoolExecutor(max_workers=8) as executor:
        batch = []
        for key in all_infection_entity_keys:
            batch.append(key)
            if len(batch) >= 500:
                delete_entities_batch(batch)
                future = executor.submit(delete_entities_batch, batch)
                futures_queue.put(future)
                batch = []

        future = executor.submit(delete_entities_batch, batch)
        futures_queue.put(future)


def consumer(futures_queue):
    client = datastore.Client()
    query = client.query(kind="infection")
    all_infection_entity_keys = (entity.key for entity in query.fetch())

    total_deleted = 0

    while not futures_queue.empty():
        future = futures_queue.get()
        result = future.result()
        total_deleted += result
        print(f"Deleted {result} keys. Total Deleted: {total_deleted}")


def delete_entities_batch(batch):
    client.delete_multi(batch)
    return len(batch)


if __name__ == "__main__":
    # Create the shared queue and launch both threads
    q = Queue()
    t1 = Thread(target=consumer, args=(q,))
    t2 = Thread(target=producer, args=(q,))
    t1.start()
    t2.start()
