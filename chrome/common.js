function goToReadingList(cb) {
  chrome.storage.sync.get('url', function(items) {
    if (!items.url) {
      alert("Please set a URL on the options page.");
    } else {
      chrome.tabs.create({url: items.url});
      if (cb)
        cb();
    }
  });
}

function addPage(kindle, cb) {
  chrome.storage.sync.get(['url', 'token'], function(items) {
    if (!items.url) {
      alert("Please set a URL on the options page.");
    } else {
      var vars = {
        url: items.url,
        token: items.token,
        kindle: kindle
      };
      chrome.tabs.executeScript({ code: 'var aread = ' + JSON.stringify(vars) }, function() {
        chrome.tabs.executeScript({ file: 'add.js' }, function() {
          if (cb)
            cb();
        });
      });
    }
  });
}
