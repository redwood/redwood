FROM ruby:onbuild
MAINTAINER Adrian Perez <adrian@adrianperez.org>
VOLUME /usr/src/app/source
EXPOSE 4567
CMD ["bundle", "exec", "middleman", "server", "--force-polling"]
