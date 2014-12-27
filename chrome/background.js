chrome.commands.onCommand.addListener(function(command) {
  if (command == "send-to-kindle") {
    addPage(true);
  }
});
