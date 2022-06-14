// Copyright 2022 Daniel Erat.
// All rights reserved.

import { $ } from './common.js';
import crypto from './sha1.js';

function saveOptions() {
  const items = {};

  const url = $('url').value.trim();
  if (url.endsWith('/')) url = url.slice(0, -1);
  items.url = url;

  const username = $('username').value.trim();
  const password = $('password').value.trim();
  if (username && password) {
    items.token = String(crypto.SHA1(`${username}|${password}`));
  }

  chrome.storage.sync.set(items);
}

function loadOptions() {
  chrome.storage.sync.get({ url: '' }).then((items) => {
    $('url').value = items.url;
  });
}

loadOptions();
$('save').addEventListener('click', () => saveOptions());
