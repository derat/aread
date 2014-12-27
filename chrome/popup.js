document.getElementById('read-later').addEventListener('click', function() { addPage(false, window.close); });
document.getElementById('send-to-kindle').addEventListener('click', function() { addPage(true, window.close); });
document.getElementById('reading-list').addEventListener('click', function() { goToReadingList(); window.close(); });
