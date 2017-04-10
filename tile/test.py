#!/usr/bin/env python

import json
import os
import subprocess
import sys
import tempfile
import uuid
import urllib2

def _run_command(cmd, out_log=None, err_log=None):
    out_fptr = None
    err_fptr = None
    try:
        if out_log is not None:
            out_fptr = open(out_log, "w")
        if err_log is not None:
            err_fptr = open(out_log, "w")

        p = subprocess.Popen(cmd, stdout=out_fptr, stderr=err_fptr, shell=True)
        rc = p.wait()
        if rc != 0:
            raise Exception("The command %s failed" % cmd)
    finally:
        if out_fptr is not None:
            out_fptr.close()
        if err_fptr is not None:
            err_fptr.close()


class OrgSpace(object):
    def __init__(self, stardog_url, stardog_password, stardog_username="admin"):
        self._orgname = "stardogtestorg%s" % str(uuid.uuid4()).split("-")[0]
        self._spacename = "stardogtestspace%s" % str(uuid.uuid4()).split("-")[0]
        self._sd_url = stardog_url
        self._sd_username = stardog_username
        self._sd_password = stardog_password
        self._service_name = None

    def set_target(self):
        _run_command("cf create-org %s" % self._orgname)
        _run_command("cf target -o %s" % self._orgname)
        _run_command("cf create-space %s" % self._spacename)
        _run_command("cf target -s %s" % self._spacename)

    def get_service_name(self):
        return self._service_name

    def clean_up(self):
        try:
            if self._service_name is not None:
                self.delete_service()
        except Exception as ex:
            print("WARN: failed to cleanup %s" % ex) 
        try:
            _run_command("cf delete-org -f %s" % self._orgname)
        except Exception as ex:
            print("WARN: failed to cleanup %s" % ex)

    def create_service(self):
        doc = {
            "url": self._sd_url,
            "username": self._sd_username,
            "password": self._sd_password,
        }
        fd, tmp_path = tempfile.mkstemp()
        os.close(fd)
        try:
            with open(tmp_path, "w") as fptr:
                json.dump(doc, fptr)
            service_name = "stardogtestservice%s" % str(uuid.uuid4()).split("-")[0]
            _run_command("cf create-service Stardog perinstance %s -c %s" % (service_name, tmp_path))
            self._service_name = service_name
        finally:
            os.remove(tmp_path)

    def delete_service(self):
        _run_command("cf delete-service -f %s" % self._service_name)


class TestApp(object):
    def __init__(self, root_dir, service_name, domain_name):
        self._test_name = "sdtestapp%s" % str(uuid.uuid4()).split("-")[0]
        self._service_name = service_name
        self._test_dir = os.path.join(root_dir, "testapp")
        self._app_url = "http://" + self._test_name + "." + domain_name

    def _make_manifest(self, manifest_path):
        manifest = """---
applications:
  - name: %s
    command: testapp
    buildpack: https://github.com/cloudfoundry/go-buildpack.git
    memory: 512MB
    instances: 1
    path: .
    services:
    - %s""" % (self._test_name, self._service_name)
        with open(manifest_path, "w") as fptr:
            fptr.write(manifest)

    def push_test_app(self):
        fd, manifest_path = tempfile.mkstemp()
        os.close(fd)
        self._make_manifest(manifest_path)
        try:
            _run_command("cf push %s -p %s -f %s" % (self._test_name, self._test_dir, manifest_path))
        finally:
            os.remove(manifest_path)

    def unbind(self):
        _run_command("cf unbind-service %s %s" % (self._test_name, self._service_name))

    def delete(self):
        _run_command("cf delete -f %s" % (self._test_name))

    def get_test_vcap(self):
        return json.load(urllib2.urlopen(self._app_url))


def test_one():
    stardog_url = os.environ['STARDOG_URL']
    stardog_pw = os.environ['STARDOG_PW']
    repo_dir = os.environ['STARDOG_SERVICE_REPO_DIR']
    domain_name = os.environ['CF_DOMAIN_NAME']
    org_space = OrgSpace(stardog_url, stardog_pw)
    try:
        org_space.set_target()
        org_space.create_service()

        test_app = TestApp(repo_dir, org_space.get_service_name(), domain_name)
        test_app.push_test_app()
        vcap_doc = test_app.get_test_vcap()
        print("%s" % str(vcap_doc))
        found = False
        for s_list in vcap_doc['Stardog']:
            if s_list['name'] == org_space.get_service_name():
                found = True
                if s_list['credentials']['url'] != stardog_url:
                    raise Exception("The stardog url doesn't match")
        if not found:
            raise Exception("The service name %s was not in the vcap" % org_space.get_service_name())
    finally:
        org_space.clean_up()


def main():
    try:
        test_one()
        rc = 0
    except Exception as ex:
        print("Failed to run the tests %s" % str(ex))
        rc = 1
    return rc


if __name__ == "__main__":
    exit_rc = main()
    sys.exit(exit_rc)
