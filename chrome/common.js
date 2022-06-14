// Copyright 2022 Daniel Erat.
// All rights reserved.

export const $ = (id) => document.getElementById(id);

// Opens the reading list in a new tab.
export function openReadingList() {
  return chrome.storage.sync.get('url').then((items) => {
    if (items.url) chrome.tabs.create({ url: items.url });
    else return Promise.reject('URL must be set in options');
  });
}

// Injected into pages by addPage().
function add(url, token, options = {}) {
  const page = options.url || document.URL;
  let req = `${url}/add?u=${encodeURIComponent(page)}&t=${token}`;
  if (options.archive) req += '&a=1';
  if (options.kindle) req += '&k=1';
  console.log(`XXX ${req}`);

  // TODO: Open the page in the background if options.url was supplied, maybe.
  // I initially used the approach here to do that when adding a link:
  // https://stackoverflow.com/questions/10812628/open-a-new-tab-in-the-background
  // It apparently doesn't work anymore, though.
  window.location.href = req;
}

// Adds a page to aread.
//
// |options| may contain the following properties:
// - 'archive' indicates that the page should be marked as read.
// - 'kindle' indicates that the page should be emailed to the Kindle gateway.
// - 'url' is the URL to add; if missing, the current URL is used.
export function addPage(options = {}) {
  return chrome.storage.sync.get(['url', 'token']).then(async (items) => {
    if (!items.url) return Promise.reject('URL must be set in options');
    if (!items.token) return Promise.reject('Token must be set in options');

    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    chrome.scripting.executeScript({
      target: { tabId: tabs[0].id },
      func: add,
      args: [items.url, items.token, options],
    });
  });
}
