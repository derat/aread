function goToReadingList() {
  chrome.storage.sync.get('url', function(items) {
    if (!items.url)
      alert("Please set a URL on the options page.");
    else
      chrome.tabs.create({url: items.url});
    window.close();
  });
}

function addPage(kindle) {
  chrome.storage.sync.get(['url', 'token'], function(items) {
    if (!items.url) {
      alert("Please set a URL on the options page.");
    } else {
      var tokenParam = items.token ? '&t=' + encodeURIComponent(items.token) : '';
      var kindleParam = kindle ? '&k=1' : ''
      chrome.tabs.executeScript({
        code: 'window.location.href="' + items.url + '/add?u="+encodeURIComponent(document.URL)+"' + tokenParam + kindleParam + '"'
      });
    }
    window.close();
  });
}

document.getElementById('read-later').addEventListener('click', function() { addPage(false) });
document.getElementById('send-to-kindle').addEventListener('click', function() { addPage(true) });
document.getElementById('reading-list').addEventListener('click', goToReadingList);
