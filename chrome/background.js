// Copyright 2022 Daniel Erat.
// All rights reserved.

import { addPage } from './common.js';

chrome.commands.onCommand.addListener((command) => {
  if (command === 'save-page') {
    console.log('Saving current page from hotkey');
    addPage({ archive: true });
  } else if (command === 'send-to-kindle') {
    console.log('Sending current page from hotkey');
    addPage({ kindle: true });
  }
});
