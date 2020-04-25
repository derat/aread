function saveOptions() {
  var items = {};

  var url = document.getElementById('url').value.trim();
  if (url[url.length - 1] == '/') url = url.slice(0, -1);
  items.url = url;

  var username = document.getElementById('username').value.trim();
  var password = document.getElementById('password').value.trim();
  if (username && password)
    items.token = String(CryptoJS.SHA1(username + '|' + password));

  chrome.storage.sync.set(items, function() {
    window.close();
  });
}

function loadOptions() {
  chrome.storage.sync.get({url: ''}, function(items) {
    document.getElementById('url').value = items.url;
  });
}

document.addEventListener('DOMContentLoaded', loadOptions);
document.getElementById('save').addEventListener('click', saveOptions);
