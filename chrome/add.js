var newWindow = false;
var page = document.URL;

if (document.URL == 'https://go-read.appspot.com/') {
  var links = document.getElementsByTagName('a');
  for (var i = 0; i < links.length; i++) {
    var link = links[i];
    if (link.getAttribute('ng-bind') == 's.Title') {
      newWindow = true;
      page = link.getAttribute('href');
      break;
    }
  }
}

var url = aread.url + '/add?u=' + encodeURIComponent(page) +
    '&t=' + aread.token + (aread.kindle ? '&k=1' : '');
if (newWindow)
  window.open(url);
else
  window.location.href = url;
