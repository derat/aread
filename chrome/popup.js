// addPage() and goToReadingList() run async, so pass window.close to be run on completion.
document.getElementById('read-later').addEventListener('click', function() { addPage(null, false, window.close); });
document.getElementById('send-to-kindle').addEventListener('click', function() { addPage(null, true, window.close); });
document.getElementById('reading-list').addEventListener('click', function() { goToReadingList(window.close); });
