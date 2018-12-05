FROM ubuntu:18.04 
ENV TEST test 
RUN echo "export WORK=yadayada" >> ~/.bashrc
CMD ["/bin/bash"]

# docker run -d dkgnode -ipAddress 123.123.123 -privateKey testtest && tail -f /dev/null
