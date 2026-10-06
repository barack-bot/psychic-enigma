var SLIDE_IMAGES = [
  'images/rtvc1.jpg',
  'images/plumbing.jpg',
  'images/electrical.jpg'
];
var FALLBACK_IMAGES = [
  'https://images.unsplash.com/photo-1523240795612-9a054b0db644?q=80&w=1920&auto=format&fit=crop',
  'https://images.unsplash.com/photo-1571260899304-425eee4c7efc?q=80&w=1920&auto=format&fit=crop',
  'https://images.unsplash.com/photo-1580582932707-520aed937b7b?q=80&w=1920&auto=format&fit=crop'
];

(function () {
  var slidesEl = document.getElementById('hero-slides');
  var dotsEl = document.getElementById('hero-dots');
  var current = 0;
  var timer = null;
  var duration = 6000;

  SLIDE_IMAGES.forEach(function (src, index) {
    var slide = document.createElement('div');
    slide.className = 'slide' + (index === 0 ? ' active' : '');
    slide.style.backgroundImage = "url('" + src + "')";
    slide.addEventListener('error', function () {
      if (FALLBACK_IMAGES[index]) slide.style.backgroundImage = "url('" + FALLBACK_IMAGES[index] + "')";
    });
    slidesEl.appendChild(slide);

    var dot = document.createElement('button');
    dot.className = index === 0 ? 'active' : '';
    dot.setAttribute('aria-label', 'Go to slide ' + (index + 1));
    dot.addEventListener('click', function () { goTo(index); restart(); });
    dotsEl.appendChild(dot);
  });

  function goTo(index) {
    var slides = slidesEl.children;
    var dots = dotsEl.children;
    current = (index + slides.length) % slides.length;
    for (var i = 0; i < slides.length; i++) {
      slides[i].classList.toggle('active', i === current);
      dots[i].classList.toggle('active', i === current);
    }
  }
  function next() { goTo(current + 1); }
  function previous() { goTo(current - 1); }
  function restart() {
    if (timer) clearInterval(timer);
    timer = setInterval(next, duration);
  }

  document.querySelector('.hero .arrow.next').addEventListener('click', function () { next(); restart(); });
  document.querySelector('.hero .arrow.prev').addEventListener('click', function () { previous(); restart(); });
  restart();
})();
