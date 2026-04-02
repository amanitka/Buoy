pipeline {
    environment {
        registry = "amanitka/buoy"
        registryCredential = 'dockerhub_id'
        dockerImage = ''
        email_to = '$DEFAULT_RECIPIENTS'
    }
    agent any
    stages {
        stage('Build image') {
            steps {
                script {
                    dockerImage = docker.build registry + ":latest"
                }
            }
        }
        stage('Push image to registry') {
            steps {
                script {
                    docker.withRegistry( '', registryCredential ) {
                    dockerImage.push()
                    }
                }
            }
        }
    }
    post {
        success {
            emailext body: 'Check console output at $BUILD_URL to view the results. \n\n ${CHANGES} \n\n -------------------------------------------------- \n${BUILD_LOG, maxLines=100, escapeHtml=false}',
            to: "${EMAIL_TO}",
            subject: 'Build successfully finished in Jenkins: $PROJECT_NAME - #$BUILD_NUMBER'

        }
        failure {
            emailext body: 'Check console output at $BUILD_URL to view the results. \n\n ${CHANGES} \n\n -------------------------------------------------- \n${BUILD_LOG, maxLines=100, escapeHtml=false}',
            to: "${EMAIL_TO}",
            subject: 'Build failed in Jenkins: $PROJECT_NAME - #$BUILD_NUMBER'
        }
    }
}