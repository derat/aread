chrome.commands.onCommand.addListener(function(command) {
  if (command == 'send-to-kindle') {
    console.log('Adding current page from hotkey');
    addPage(null, true);
  }
});

chrome.contextMenus.onClicked.addListener(function(info, tab) {
  if (info.menuItemId == 'send-to-kindle') {
    console.log('Adding ' + info.linkUrl + ' from context menu');
    addPage(info.linkUrl, true);
  }
});

chrome.contextMenus.create({
  id: 'send-to-kindle',
  title: 'Send to Kindle',
  contexts: ['link']
});
