var page = null;
var newTab = false;

// Awfulness courtesy of
// http://stackoverflow.com/questions/736513/how-do-i-parse-a-url-into-hostname-and-path-in-javascript
var a = document.createElement("a");
a.href = document.URL;
var hostname = a.hostname;

if (hostname == 'go-read.appspot.com' || hostname == 'www.goread.io') {
  var links = document.getElementsByTagName('a');
  for (var i = 0; i < links.length; i++) {
    var link = links[i];
    if (link.getAttribute('ng-bind') == 's.Title') {
      page = link.getAttribute('href');
      newTab = true;
      break;
    }
  }
  if (page == null)
    alert("Page link not not found.");
} else {
  page = document.URL;
}

if (page != null) {
  var url = aread.url + '/add?u=' + encodeURIComponent(page) +
      '&t=' + aread.token + (aread.kindle ? '&k=1' : '');
  if (newTab) {
    // Ought to use chrome.tabs.create() here, except this is injected into the page.
    // Instead, use this lovely code from http://stackoverflow.com/questions/10812628/open-a-new-tab-in-the-background
    var a = document.createElement("a");
    a.href = url;
    var e = document.createEvent("MouseEvents");
    // The tenth parameter of initMouseEvent sets ctrl key.
    e.initMouseEvent("click", true, true, window, 0, 0, 0, 0, 0, true, false, false, false, 0, null);
    a.dispatchEvent(e);
  } else {
    window.location.href = url;
  }
}
