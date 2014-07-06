function saveOptions() {
  var url = document.getElementById('url').value;
  var token = document.getElementById('token').value;
  chrome.storage.sync.set({
    url: url,
    token: token
  }, function(){});
}

function loadOptions() {
  chrome.storage.sync.get({url: '', token: ''}, function(items) {
    document.getElementById('url').value = items.url;
    document.getElementById('token').value = items.token;
  });
}

document.addEventListener('DOMContentLoaded', loadOptions);
document.getElementById('save').addEventListener('click', saveOptions);
