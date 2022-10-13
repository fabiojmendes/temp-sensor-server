/* Requires the Docker Pipeline plugin */
pipeline {
  agent { docker { image 'golang:1.19.1-alpine' } }
  environment {
    GOCACHE = '/tmp/'
  }
  stages {
    stage('check env') {
      steps {
        sh 'pwd'
        sh 'ls -la '
      }
    }
    stage('build producer') {
      environment {
        GOOS = 'linux'
        GOARCH = 'arm'
        GOARM = 6
      }
      steps {
        sh 'go build -o build/ ./src/tsproducer'
      }
    }
    stage('build consumer') {
      environment {
        GOOS = 'linux'
        GOARCH = 'amd64'
      }
      steps {
        sh 'go build -o build/ ./src/tsconsumer'
      }
    }
  }
}
