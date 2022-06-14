// Copyright 2022 Daniel Erat.
// All rights reserved.

import { $, openReadingList, addPage } from './common.js';

$('save-page').addEventListener('click', () =>
  addPage({ archive: true })
    .then(window.close)
    .catch((e) => alert(e))
);
$('read-later').addEventListener('click', () =>
  addPage()
    .then(window.close)
    .catch((e) => alert(e))
);
$('send-to-kindle').addEventListener('click', () =>
  addPage({ kindle: true })
    .then(window.close)
    .catch((e) => alert(e))
);
$('reading-list').addEventListener('click', () =>
  openReadingList()
    .then(window.close)
    .catch((e) => alert(e))
);
