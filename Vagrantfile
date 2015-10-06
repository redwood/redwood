Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/trusty64"
  config.vm.network :forwarded_port, guest: 4567, host: 4567

  config.vm.provision "bootstrap",
    type: "shell",
    inline: <<-SHELL
      sudo apt-get update
      sudo apt-get install -yq ruby ruby-dev build-essential nodejs git
      sudo apt-get autoremove -yq
      gem install --no-ri --no-rdoc bundler
    SHELL

  config.vm.provision "install",
    type: "shell",
    privileged: false,
    inline: <<-SHELL
      echo "=============================================="
      echo "Installing app dependencies"
      cd /vagrant
      bundle install
    SHELL

  config.vm.provision "run",
    type: "shell",
    privileged: false,
    inline: <<-SHELL
      echo "=============================================="
      echo "Starting up middleman at http://localhost:4567"
      echo "If it does not come up, check the ~/middleman.log file for any error messages"
      cd /vagrant
      bundle exec middleman server --force-polling -l 1 &> ~/middleman.log &
    SHELL
end
